# Recommended architectures

[Docs](README.md)

Waffle scaffolds one tree. A domain written as the second of twenty in a single
binary and a domain written as the only one in its own service are the same
eleven files — `waffle make:domain <domain>` does not ask which you are building.

So the choice on this page is not a choice of layout. What actually differs
between the three shapes is which modules the composition root imports, where
migrations run, and whether a call from one domain to another crosses a network.
This page is which shape to pick, what each costs, and what it takes to change
your mind later.

## The three shapes

| Shape | When | Deployed as | A cross-domain call is |
|---|---|---|---|
| **Single service** | one domain, a handful of endpoints | one binary, one role | not a thing yet |
| **Modular monolith** | many domains, one team or a few, one database | one binary, several roles by argv | a function call through an interface |
| **Service fleet** | many domains, many teams, a database each | many binaries | gRPC, HTTP or a queue |

The middle row is the one the rest of this page is about, because it is the one
waffle's machinery is shaped for.

## What every shape shares

```
cmd/app/main.go          entry point — builds the container, calls RunUntilStop
config/di/               hand-written registrations, di_<name>_*.go per domain
config/di/modules.go     blank imports: the set of modules this binary carries
config/diogen/           generated container (di_gen.go, do not edit) and its adapter
domain/<name>/
  model/                 entities and domain errors
  service/               business logic, and the interfaces it needs
  adapter/postgres/      repository implementation, stored structs
  cmd/migrate/           migration runner for this domain
api/                     proto definitions, API init
```

Three things in that tree do the work of this page, and it is worth naming them
before the shapes diverge.

**The service declares the interface; something else implements it.** What
`waffle make:domain <domain>` writes is a `XRepository` interface *in the service
package* and a postgres adapter that satisfies it. The service never imports the
adapter. That inversion is not a database idiom — the database is just the first
foreign system a domain meets.

**A runtime concern arrives by blank import.** `config/di/modules.go` is the list
of what this binary can do. Adding a transport is a line; removing one is
deleting a line. See [Introduction](introduction.md) for the built-in set.

**One binary, many roles.** `./app http`, `./app grpc`, `./app migrate up`,
`./app scribus …` — the role is argv, not a separate artifact. See
[Commands](core/commands.md).

## Single service

One domain, three to ten endpoints, no second reader.

Be honest about this case: most of waffle is overhead here. A scaffolding CLI to
save a `main.go` you would have written in an afternoon is not a trade that pays,
and a small Go service next to a large Python one is the case waffle explicitly
does not fit.

What still pays on its own, without adopting the rest, is the core:
`failure`, `pry`, `session` and `hilton` give you typed errors, correlation ids
across transports and one instrumentation vocabulary. Each is an ordinary import
with no framework attached. [What is optional](opting-out.md) lists what every
other tool replaces and what skipping it costs.

## Modular monolith

Many domains, one binary, one database. The bet is that twenty domains read the
same way because the scaffolding made them, and that you pay for exactly one
deployment, one schema and one transaction.

### A module is a domain directory

`domain/<name>/`, migrations included, plus its registration files in `config/di/`
and, if it speaks gRPC, its proto under `api/proto/<name>/`. Nothing else belongs
to it, and nothing of it lives anywhere else.

### The boundary rule

A modular monolith is a ball of mud with good intentions unless something
refuses the import that would flatten it. The rule:

Nothing outside a domain may reach into it. Three exceptions, and no others:

| Importer | May reach `domain/Y` |
|---|---|
| `domain/X/adapter/**` | **`domain/Y/model`, and nothing else of `Y`** |
| `domain/X/model`, `service`, `cmd/**` | no |
| a composition root | everything — it wires the graph by definition |
| `tests/**` | everything — it boots the real container |
| `pkg/**`, `api/`, anything else | no |

The last row is the point of stating the rule this way round: it is about being
outside every domain, not about a directory anyone thought to name. A `lib/`
invented next week is refused without the rule being edited.

A composition root is whatever carries a `//go:generate diogen` line, plus the
directory its `-output` writes to — the module root when the line names none,
which is diogen's own default. No path is spelled anywhere in the check, so a
service with two containers gets both wherever they sit, and a domain that
registers itself is exempt inside its own tree.

That finds a root by the generator's own contract, so the test `waffle
init:service` writes names nothing at all:

```go
func TestDomainsStayBehindTheirAdapters(t *testing.T) {
    if err := boundaries.Check(); err != nil {
        t.Fatal(err)
    }
}
```

A service that [skips diogen](opting-out.md#diogen) has no such contract to be
found by, and says where composition lives in that same line:

```go
    if err := boundaries.Check(boundaries.Roots("config/di")); err != nil {
```

That option is not an exception list. It states a fact about the tree, in the
service's own source, in the diff of whoever moved it.
Nothing is granted by it that `config/di` did not already have.

Whatever the root imports is its business; what it may **wire** is the rule in
the next section.

The adapter is where a domain meets a foreign system. Postgres is one such
system; another domain is another. Letting the adapter package reach across —
and only it — puts every cross-domain dependency in a directory named after the
fact that it is one.

Inside a domain the same idea closes two more doors: `adapter/**` is imported
only from another adapter, and `cmd/**` only from inside `cmd/**`. A service
that names its own adapter has welded itself to it, and a transport is nobody's
business but its own. `model/` stays open to the whole domain.

So a domain that needs something from another domain does what it already does
for its database:

```go
// domain/orders/service — orders owns the interface it needs
type Catalog interface {
    Price(ctx context.Context, sku string) (int, error)
}
```

`domain/orders/adapter/catalog` implements it — and reaches the neighbour the
same way, one layer down: it declares the interface *it* needs, imports
`domain/catalog/model` for the types that cross, and translates them into its
own. The composition root answers that interface with `domain/catalog/service`
and wires the adapter into `orders/service`.

So neither package imports `catalog/service`. `orders/service` never learns that
`catalog` exists at all, and the adapter knows only a shape it declared itself —
which is why the table above lets an adapter reach a neighbour's `model` and
nothing else of it.

### Why the rule is a linter and not `internal/`

The obvious move is to hide a domain's packages under `domain/<name>/internal/`
and let the compiler do it. It does not work here, and the reason is worth
knowing before someone tries it again.

Go's rule is that `domain/Y/internal/…` is importable only from packages rooted
at `domain/Y/`. The composition root is `config/di`, which is *above* the
domains, not under them — so hiding a domain from its siblings hides it from the
container that has to construct it. The shape you need is "visible to the root
and to myself, invisible to my siblings", and `internal/` cannot express it:
it walls off a subtree, and the root is not in the subtree.

The alternatives are worse. `internal/domain/Y/` walls the project off from the
outside world and does nothing between domains. A Go module per domain does work,
at the price of a `go.mod` and a `replace` per domain inside one repository.

So the boundary is a check over the import graph, with the layer table above as
its rule. `boundaries.Check()` runs it as an ordinary test from
`config/diogen/boundaries_test.go`, which `waffle init:service` writes.

There is no exception list, and that is deliberate. A crossing that looks like it
needs an entry is a design question, and a format invented before the first
exception guesses wrong.

### What the import graph stops telling you

After the rule is in place, `domain/orders/service` imports nothing from
`catalog` — which was the point, and which means the import graph no longer
answers "which domains depend on which". Only the container knows, because the
edge now exists solely as a wiring decision in `config/di`.

Keep that in mind when you want the real module graph: read it off the generated
container, not off the imports.

The check reads it too, and has to. A borrowed method whose signature is all
primitives — `Cancel(ctx, id) error` — can be declared in `orders/service` and
wired to `catalog/service` with no import anywhere, so the import half is blind
to it by construction. The second half reads `di.Wire`, `di.Define` and
`di.Alias`: a registration may answer one domain's interface with another
domain's package only when the interface is in `domain/X/adapter/**` and the
answer is `domain/Y/service`. Types cross by import, behaviour crosses by
registration, and neither half sees the other's traffic.

A handle counts as its constructor. `di.Define` hands one back, a second file
wires it, and the registration then names no package at all — which is the
spelling every scaffolded transport uses, so the check resolves a handle to the
package it was defined from before it judges the crossing. Package scope, not
file: the handle is declared in `di_<domain>_domain.go` and used from the file
beside it.

### Events between modules

`dispatcher` is in-process typed event dispatch with a priority and no broker —
the tool for "something happened in `orders`, `billing` cares" when both are in
the same binary.

One sharp edge: `Dispatch` returns `nil` when nothing is subscribed. A forgotten
blank import is a silent no-op, not an error. In a twenty-module binary that is
the bug you will look for the longest, so treat a subscriber registration as
something a test asserts, not something you assume.

### Events leaving the binary

`outbox` writes the event in the same transaction as the change that caused it
and relays it afterwards. With one database this is the whole of the problem:
there is nothing to make consistent across a boundary, so the outbox is the
boundary. `waffle make:outbox --driver <id>` enables it and writes the table.
See [Outbox](tools/outbox.md).

### Schema

Migrations stay per domain — `waffle make:domain:migrate <domain>` gives each its
own runner, and `./app migrate up` runs them all. One database, many owners.

Nothing stops one domain's SQL from joining another's table, and no check
proposed here would see it: it is a string, not an import. The discipline that
holds is the same one as everywhere else — the adapter is the only place that
writes SQL, so the only place a foreign table name can appear is a file already
marked as reaching outward.

### Testing

A [component test](di/overview.md#testing-with-the-container) boots the real
container over real infrastructure and replaces only the leaves. In a monolith
that is per-module component tests against the same binary — the thing everybody
wants from a modular monolith and rarely gets, because there is usually no seam
to boot a module through.

### What is not there yet

One gap, so the page does not promise what the code does not hold:

- **A domain's wiring lives centrally.** `waffle make:domain <domain>` writes
  `config/di/di_<domain>_domain.go`, not a registration package inside the
  domain — so a domain is not quite a detachable unit. The pattern that would
  make it one is already in the framework: every waffle tool registers itself in
  its own package and is pulled in by a blank import.

## Service fleet

Many domains, many teams, a database per service. This is the shape the rest of
the documentation assumes.

What changes from the monolith is narrow: `config/di/modules.go` carries fewer
modules per binary, each service owns its schema, and a call that was a function
call becomes gRPC, HTTP or a queue message. The domain tree does not move.

What it buys is independent deploys and team autonomy. What it costs is that
every cross-domain call is now a network call with its own failure mode, and
`delay`'s in-process saga stops being enough — a crash mid-chain leaves nothing
to resume from, which is survivable inside one transaction and is not survivable
across four services.

## Changing your mind

The reason to start as a modular monolith is that leaving it is cheap, and the
boundary rule is what makes it cheap.

| What moves | Monolith → fleet |
|---|---|
| `domain/<name>/` | moves as-is, unchanged |
| the service package | unchanged — it only ever knew its own interfaces |
| `domain/X/adapter/<other domain>` | the one file that changes: a local call becomes a client call |
| `config/di/modules.go` | loses the lines for what this binary no longer does |
| migrations | follow the domain |
| the database | splits, and this is the real work |

The adapter is the whole trick. It was already the only package that knew
another domain existed, so it is the only package that has to learn the
difference between a function call and an RPC. That is worth more than the
tidiness the rule buys on its own — it is the reason the decision stays
reversible.

Going the other way — collapsing a fleet into a monolith — is the same table
read upwards, and it is easier, because merging schemas is a migration and
splitting them is a project.

## Choosing

| If | Then |
|---|---|
| one domain, and it will stay one | single service, and take only the core |
| many domains, one team, one database | modular monolith |
| many domains, several teams that deploy on their own schedules | fleet |
| you cannot tell yet | modular monolith — the table above is the exit |
