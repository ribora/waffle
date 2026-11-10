# What each tool replaces, and what you lose by skipping it

[Docs](README.md)

Waffle is opinionated about the **path**, not about the tools on it. The entry
point takes three things:

```go
func Init(commands []Command, logger pry.Logger, provider hilton.Provider) (*app, error)
```

A logger, a tracing provider, and a list of commands. Everything else — the DI
generator, the SQL layer, the input codegen, every transport — is something you
opt into with a blank import, and can leave out.

This page exists because the value of a tool is measured in what does *not*
happen: a panic that never fires, a parser you never write. Those are invisible
if you join a project where everything is already generated. Below, each tool is
paired with the code you write instead.

## The core you cannot really remove

Four packages are imported by the platform itself, not just wired into it:

| Package | Imported by | Why it is core |
|---|---|---|
| `hilton` | `amqp`, `elastic`, `grpc`, `http`, `redis`, `sql`, `waffle` | every package emits spans and metrics through it; removing it means every package loses its instrumentation |
| `failure` | `amqp`, `grpc`, `http`, `pry`, `redis`, `waffle`, `whisk` | the shared error shape — what an interceptor maps to a status code, what a logger records |
| `pry` | `amqp`, `elastic`, `grpc`, `http`, `waffle` | the logger the framework hands to every component |
| `session` | `amqp`, `grpc`, `http`, `pry`, `waffle` | request/command identity — correlation ids across transports |

`whisk/scan` sits with them in one narrow sense: `http` and `amqp` import the
`Source` interface, because that is the contract an adapter implements. The
whisk **generator** is a separate choice — see below.

Everything else in the table below is imported by `waffle` and nothing more.

## Everything else

| Tool | What it replaces | Skip it and you | What you lose |
|---|---|---|---|
| `diogen` | wiring written and ordered by hand | write `InitDi()` yourself | conditional wiring, collections across packages, modules by blank import |
| `whisk` | request parsing and validation by hand | parse in the handler, use `server.Handle` | the schemas in the OpenAPI document, one binding for every transport |
| `sql` | any query layer | use `pgx`, `sqlc`, `bun`, whatever | the repository generics and the `stored<X>` mapping |
| `http`, `grpc`, `scribus`, `outbox`, `cron`, `migrate` | a server or consumer you start yourself | implement `Command` directly | the wiring, flags and graceful shutdown each tool brings |
| `elastic`, `redis` | thin clients | use the official ones | nothing but the constructors |
| `goverter` | conversions between DTO and domain | write them by hand (`mapper: dto`) | the refusal to generate on an unmappable field — a hand-written mapper compiles happily while a new model field stops reaching the wire |

## diogen

**What it does not buy you.** Compile-time safety over hand-written wiring:
Go already provides that. If a constructor needs a `UserRepository` and you pass
nothing, the build fails with or without a generator.

The relevant comparison is against a **reflection container**. There, a missing or
ambiguous dependency is a panic at startup rather than a build error.

**What it does buy you**, compared to writing `InitDi()` by hand:

- **Order.** You do not maintain construction order. Add a dependency to a
  constructor and nothing else moves.
- **Conditions.** Wiring that depends on the environment stays readable:

  ```go
  if env.Value("CACHE", "") == "redis" {
      di.Wire[Cache](NewRedisCache)
  } else {
      di.Wire[Cache](NewMemoryCache)
  }
  ```

  By hand this is an `if` around a construction block, and every consumer below
  it has to be built inside both branches, or hoisted and reassigned.
- **Collections across packages.** `di.Tag` / `di.Tags` gather every
  implementation registered under a tag, including from packages your service
  only blank-imports. By hand, every command and router is a line in a slice
  literal that somebody has to remember to update.
- **Modules.** A framework package ships its own registrations; a service opts
  in with one blank import. By hand, adding a tool means copying its wiring into
  your `InitDi()` and re-copying it on every upgrade.

**Skipping it** costs nothing structural: write `InitDi() (app.App, error)`,
construct what you need in order, and call `app.Init(commands, logger,
provider)`. The generated container is not special — `config/diogen/di.go` is a
hand-written adapter over it, and you can replace both.

Two things about the [boundary rule](architectures.md#the-boundary-rule). It
finds a composition root by diogen's `//go:generate` line, so without one there
is nothing to find — name yours in the test that runs the check,
`boundaries.Check(boundaries.Roots("config/di"))`, and it reaches every domain as
before. And it loses the half of the rule that reads registrations: with no
`di.Wire` to read, a cross-domain dependency written as a hand-rolled constructor
call is caught only if it also crosses by import.

## whisk

**Instead of** tags on a struct:

```go
type createOrder struct {
    Currency string `scan:"query=currency" validate:"required,currency"`
    Email    string `json:"email"          validate:"required,email"`
}
```

**you write** the body decode, the query lookup, the type conversion and each
check, per handler, per field — and repeat it for the AMQP consumer that accepts
the same payload from a different source.

**Skipping it** is a supported path: register routes with `server.Handle`
instead of `server.Bind`, read `src.Body()` and unmarshal yourself.

**What you lose:** the schemas in the OpenAPI document. They are derived from the
request and response types captured by `Bind`; a `Handle` route has no types to
capture, so it reaches the document as a path, its path parameters and a bare 200.
You also lose the one-binding-per-DTO property — the same struct read from an HTTP
request and an AMQP delivery.

## sql

Nothing in the platform requires it. Only `waffle` imports it, to wire a connection
and for the `migrate` and `outbox` tools that run over one. Adapters are your code:
use `pgx` directly, `sqlc`, `bun` or anything else, and the repository interface
your service declares stays yours.

What you give up is the `sql.Repository[Model]` generics and the `stored<X>`
struct whose `Scan`/`Inserts`/`Updates` methods are generated from tags.

## Transports

`http`, `grpc`, `scribus`, `outbox`, `cron` and `migrate` are `Command`
implementations pulled in by blank import. A service that needs none of them is a valid waffle
service — `app.Init` accepts an empty command list. A service that needs
something waffle does not ship implements `Command` itself; see
[core/commands](core/commands.md).

## Walking back out

Everything above is about not adopting a tool. This section covers un-adopting one
after you have.

**A generator.** Delete its `//go:generate` line. The file it wrote stays exactly
where it is and becomes ordinary source you edit by hand: nothing regenerates it,
nothing validates it, no build step looks for it.

For containers the break is total — **no generated container imports
`gitlab.com/gorib/di`**. Every non-standard import in `di_gen.go` is a package
whose constructors it calls, yours and the framework's, so what remains after
deleting the directive has no link back to the tool that wrote it.

For whisk it costs a copy. Generated `Parse` and `Validate` keep referring to two
of whisk's packages, and to keep them compiling on your own you copy:

| Copy | Lines | What for |
|---|---|---|
| `scan/source.go` | 59 | the `Source` interface — and ~20 of those are a bytes-backed implementation for tests |
| `scan/errors.go` | 6 | `ErrPayload`, over `failure` — core, and imported by a generated `Parse` as well |
| `validate/validation_error.go` | 151 | `ValidationError`, `Path`, the segment constructors and the formatters |
| the rules you actually call | 7–117 each | `IsEmail`, `IsUuid`, … — only the ones your tags name |

So a `Parse` survives on 65 copied lines, a `Parse` plus `Validate` on about 220.
None of it is logic you have to understand: it is the shape of a request and the
shape of an error. `whisk.yaml` and the `tool` line in `go.mod` go with the
directive — nothing else reads them.

**Waffle itself.** Write your own `main`, call the container's `InitApp`, and drop
the `config/*` module imports you no longer want. What you keep is a plain
constructor graph; what you give up is the command lifecycle, the standard
middleware chain and graceful shutdown — every one of which arrived as a `Command`
by blank import rather than by inheritance. Directory layout and architecture are
yours from that point on.

There is no build step here you cannot delete, and no generated file you cannot
take ownership of.

## What "opinionated" means here

Not that you must use these tools. That there is **one** way to do each thing
once you have chosen the tool: one way to add a runtime concern (blank import),
one way to declare input (tags plus `go:generate`), one way to extend a
generator (a function named after the thing you are adding), one shape for an
escape hatch (a lowercase method the generator finds).

The path is opinionated so that a service written by one person reads like a
service written by another. Which tools are on your path is a separate question,
and the answer is allowed to be "fewer".
