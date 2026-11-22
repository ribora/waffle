# Input Parsing and Validation

[Docs](../README.md) · Conventions

Every entry point reads structured input from somewhere — an HTTP request, an AMQP delivery, a file on disk. `github.com/ribora/whisk` generates the parsing and validation code for all of them from struct tags: `Parse(scan.Source) error` to populate the struct, `Validate() error` to check it. No runtime reflection, and no transport baked into the DTO.

## The Source contract

```go
type Source interface {
    Lookup(scope, key string) string
    LookupAll(scope, key string) []string
    Body() []byte
    Context() context.Context
}
```

A **scope** is an opaque string. whisk never hardcodes one — it emits `src.Lookup("query", "status")` verbatim and the adapter decides what that means. An unknown scope returns an empty string; that is the contract, not a failure. Binding reads what the transport carries and says nothing about whether it should have been there — checking that is `validate`'s job, and it is the only place that does it.

| Adapter | Scopes | of those, `,repeated` |
|---|---|---|
| `ribora/http/server` | `path`, `query`, `header`, `cookie`, `form`, `request=method` | `query`, `header`, `form` |
| `ribora/amqp` | `header`, `routing-key`, `exchange` | all three, but each holds one value, so a slice gets at most one element |
| `scan.FromBytes` | none — body only, for files, fixtures, anything already in memory | — |

The same DTO therefore works behind any of them: a struct whose fields all come from the body needs no scope at all, and one that reads `header` reads it from an HTTP request and an AMQP delivery alike.

Which means a misspelled scope, or one belonging to another transport, is not caught anywhere: the field is simply never filled. What reports it is `validate:"required"` on that field — as a missing input, so read the tag before the caller. The `,repeated` column has the same shape: on a scope that holds one value the slice stays empty and nothing objects.

## Generating

```go
//go:generate go tool whisk $GOFILE

package http

type createOrder struct {
    TraceID  string   `scan:"header=X-Trace-Id"`
    Currency string   `scan:"query=currency"  validate:"required,currency"`
    Email    string   `json:"email"           validate:"required,email"`
    Items    []string `json:"items"           validate:"required,unique,dive,required"`
}
```

whisk writes `<file>_whisk_gen.go` next to the source. A field with a `scan` tag comes from that scope, a field without one comes from the body, and scoped lookups are applied after the body so they win.

Read it with `scan.Into`, whatever the source is — the function belongs to whisk
rather than to a transport, which is the same reason the tags do:

```go
req, err := scan.Into[createOrder](src)
```

For HTTP there is one step less: `server.Bind` runs it for you and hands the
handler the parsed request.

`Into` runs `Parse`, then `Validate` if the type has one, and returns on the first error.

## Add your own tag in a minute

Custom validators are a convention, not a configuration step. Export `Is<Tag>` from any file in `pkg/validate` at the module root:

```go
// pkg/validate/rules.go
package validate

func IsPasscode(value string, args ...string) bool {
    return len(value) == 6
}
```

Use it:

```go
type auth struct {
    Code string `validate:"required,passcode"`
}
```

Run `make generate`. That's the whole mechanism: the function is `Is` + the tag in PascalCase, the file it lives in doesn't matter, and no flag has to point at anything — `pkg/validate` is the default, written out in `whisk.yaml` when `make:domain:http` or `make:domain:scribus` first pulls whisk in. A missing directory simply means no custom validators.

Three things worth knowing:

- Return `(bool, error)` instead of `bool` when the failure has detail worth keeping; the error is stashed in `ValidationError.Err` for the caller to unwrap.
- **A custom validator whose name collides with a built-in wins.** Your `email` rule replaces whisk's without forking anything — and whisk warns when it does, since that's the one case where the model doesn't show which check runs.
- Moving the directory is a `validate.storage` line in `whisk.yaml`, or `-validate.storage` on a single `go:generate` line.

## Escape hatches

Codegen covers the shape most DTOs have. Everything else has a hook; hooks are part of the design, not a workaround.

| When | Hatch |
|---|---|
| A rule tags can't express — cross-field logic, business state, a registry lookup | `func (s T) validate() error`, runs after the generated checks; its errors merge into the same chain |
| A field type whisk can't convert, a derived value, a default | `func (s *T) parse(src scan.Source) error`, runs last, after the generated binding |
| A body format that isn't JSON or XML | an `Unmarshal<Format>` function in `pkg/scan`, selected with `-format=<name>` |
| A tag that doesn't exist yet | `Is<Tag>` in `pkg/validate`, as above |

Both hooks are lowercase methods you write by hand; whisk detects them and calls them from the generated code.

## Without whisk

The generator is optional. Below is the same handler with the codegen removed,
so the trade-off can be read off the code.

**With whisk.** Two tags per field and one directive; `Into` runs the generated
`Parse`, then `Validate`:

```go
//go:generate go tool whisk $GOFILE

type createOrder struct {
    Currency string   `scan:"query=currency" validate:"required,currency"`
    Email    string   `json:"email"          validate:"required,email"`
    Items    []string `json:"items"          validate:"required,unique,dive,required"`
}

req, err := scan.Into[createOrder](src)
```

**Without it**, the same contract in the handler:

```go
type createOrder struct {
    Currency string
    Email    string
    Items    []string
}

func parseCreateOrder(src server.Source) (*createOrder, error) {
    var body struct {
        Email string   `json:"email"`
        Items []string `json:"items"`
    }
    if err := json.Unmarshal(src.Body(), &body); err != nil {
        return nil, failure.Wrap("%w: malformed body", scan.ErrPayload)
    }

    out := &createOrder{Email: body.Email, Items: body.Items}
    out.Currency = src.Lookup("query", "currency")

    if out.Currency == "" {
        return nil, failure.Wrap("%w: currency is required", scan.ErrPayload)
    }
    if ok, _ := validate.IsCurrency(out.Currency); !ok {
        return nil, failure.Wrap("%w: currency is not a currency", scan.ErrPayload)
    }
    if out.Email == "" {
        return nil, failure.Wrap("%w: email is required", scan.ErrPayload)
    }
    if ok, _ := validate.IsEmail(out.Email); !ok {
        return nil, failure.Wrap("%w: email is not an email", scan.ErrPayload)
    }
    if len(out.Items) == 0 {
        return nil, failure.Wrap("%w: items is required", scan.ErrPayload)
    }
    seen := make(map[string]bool, len(out.Items))
    for _, item := range out.Items {
        if item == "" {
            return nil, failure.Wrap("%w: items contains an empty value", scan.ErrPayload)
        }
        if seen[item] {
            return nil, failure.Wrap("%w: items must be unique", scan.ErrPayload)
        }
        seen[item] = true
    }
    return out, nil
}
```

Three fields. The version without codegen is longer than the struct it fills,
and every rule is a place to forget a check, return the wrong status, or word
the message differently from the handler next door.

Then the same payload arrives from a queue, and `src.Body()` is the only line
that still applies — the query lookup has to become a header lookup, by hand,
in a second function.

What you give up by skipping whisk is listed in
[What is optional](../opting-out.md#whisk): the OpenAPI document is derived from
the types `server.Bind` captures, and a hand-parsed handler has none.

## Troubleshooting generation

Every generated file opens with `//go:build !whisk`, and whisk type-checks the
package with `-tags whisk`. Its own output is therefore invisible to it: no
`*_whisk_gen.go` in the directory — the one being replaced or any other — can
block the run that would fix it. Renaming a field and regenerating always works;
there is nothing to clean up first. The corollary is that `whisk` is a reserved
build tag: never pass it to your own builds, or the `Parse` and `Validate`
methods disappear.

whisk still needs the rest of the package to type-check, so a genuine compile
error in your hand-written code surfaces as the compiler's own message:

```
mymodule/domain/order/cmd/http: order.go:14:9: undefined: model.Currency
```

### Generation errors

| Message | Cause | Fix |
|---|---|---|
| `T.F: unknown validator tag "passcode": … no func IsPasscode in pkg/validate (…)` | the tag is not built in and the storage dir has no `IsPasscode` | add the validator (see above), correct the tag name, or point `validate.storage` at the right directory — the message says what it found there |
| `T.F: scan tag "…" must be "-", "embed", or "<scope>=<key>[,opt...]"` | a `scan` tag with no `=` | write `scan:"query=status"` |
| `T.F: scan tag "…" has empty scope` / `has empty key` | `scan:"=status"` / `scan:"query="` | fill both sides |
| `T.F: unknown scan option "x" (allowed: repeated)` | third comma-separated part of the tag | `repeated` is the only option |
| `T.F: scan option "repeated" requires a slice field, got string` | `repeated` maps to `LookupAll` | make the field a slice |
| `T.F: type X has scan tags but is not marked scan:"embed"` | a nested struct's `scan` tags are dead — the field is bound from the body | add `scan:"embed"`, or drop the inner tags |
| `T.F: scan:"embed" cannot be used on a slice/map field` | embed delegates to one `Parse` call | use a `parse()` hook for per-element binding |
| `T.F: scan:"embed" on a pointer field is not supported` | nothing allocates the pointee | embed by value, or allocate in a `parse()` hook |
| `T.F: scan:"embed" requires a named struct type` | there is no `Parse` method to delegate to | give the anonymous struct a name |
| `required tag doesn't make sense for bool field "F", use equal instead` | `required` would reject a legitimate `false` | `validate:"equal=true"` |
| `unique=G: cannot specify field on slice of non-struct type "X"` | `unique=Field` only applies to `[]struct` | drop the field name |
| `unique=G: anonymous struct in slice "F" has no field "G"` | typo in the field name | match the element's field |
| `wrong comparison for field F` | an ordering comparison, or a value other than `true`/`false`, on a bool field | `validate:"equal=true"` |
| `eq doesn't make sense for time field "F", an instant is bounded with gte/lte rather than matched` | equality against a `time.Time` | bound it with `gte`/`lte` |
| `T.parse must take github.com/ribora/whisk/scan.Source` / `T.validate must return error` | hook signature | `func (s *T) parse(src scan.Source) error`, `func (s T) validate() error` |
| `unknown format "yaml": no built-in preset, and no func UnmarshalYaml in pkg/scan (…)` | `-format=yaml` with nothing behind it | add the function, or drop the flag |

### Silent outcomes

**No file appeared.** whisk generates for every named struct declared in the
target file and writes nothing if the file declares none. Check that
`//go:generate go tool whisk $GOFILE` is in the file that declares the DTO, not
in a neighbour.

**`// TODO whisk: cannot convert string to X for F — handle in parse(src) hook`.**
A scoped field whose type is not a string, bool or numeric kind. The generated
file still compiles and the field stays zero, so grep for `TODO whisk` after
generating and fill it in a `parse(src)` hook.

**`whisk: warning: … embedded field F uses scan:"embed"`.** An *anonymous* embed
promotes the embedded type's generated `Parse` into the outer method set, where
it can shadow the outer one. Give the field a name.

### The generated file does not compile

| Symptom | Cause |
|---|---|
| `method T.Parse already declared` | the struct already has a hand-written `Parse`, and whisk generates for every struct in the file — move the DTO to its own file, or rename the hand-written method |
| `s.F.Parse undefined` | `scan:"embed"` on a type whose own package was never generated — add the `//go:generate` directive there too |
| `undefined: validate.IsX` | stale output: a custom validator was renamed or deleted while the tag stayed. Regenerating turns this into `unknown validator tag` |

## Where this shows up

| Context | Page |
|---|---|
| HTTP handlers and request DTOs | [tools/http-handler.md](../tools/http-handler.md) |
| Queue consumers | [tools/scribus.md](../tools/scribus.md) |
| The full tag reference — every `scan` and `validate` rule, `dive`, `embed`, error shapes | the `README.md` of `github.com/ribora/whisk` |
