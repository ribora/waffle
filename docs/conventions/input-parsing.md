# Input Parsing and Validation

Every entry point reads structured input from somewhere — an HTTP request, an AMQP delivery, a file on disk. `gitlab.com/gorib/whisk` generates the parsing and validation code for all of them from struct tags: `Parse(scan.Source) error` to populate the struct, `Validate() error` to check it. No runtime reflection, and no transport baked into the DTO.

## The Source contract

```go
type Source interface {
    Lookup(scope, key string) string
    LookupAll(scope, key string) []string
    Body() []byte
    Context() context.Context
}
```

A **scope** is an opaque string. whisk never hardcodes one — it emits `src.Lookup("query", "status")` verbatim and the adapter decides what that means. An unknown scope returns an empty string; that is the contract, not a failure.

| Adapter | Scopes |
|---|---|
| `gorib/http/server` | `path`, `query`, `header`, `cookie`, `form` |
| `gorib/amqp` | `header`, `routing-key`, `exchange` |
| `scan.FromBytes` | none — body only, for files, fixtures, anything already in memory |

The same DTO therefore works behind any of them: a struct whose fields all come from the body needs no scope at all, and one that reads `header` reads it from an HTTP request and an AMQP delivery alike.

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

Read it with `scan.Into`, or with the transport's own helper — `server.Into` and `server.Bind` for HTTP:

```go
req, err := scan.Into[createOrder](src)
```

`Into` runs `Parse`, then `Validate` if the type has one, and returns on the first error.

## Add your own tag in a minute

Custom validators are a convention, not a configuration step. Drop a file named after the tag into `pkg/validate` at the module root:

```go
// pkg/validate/passcode.go
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

Run `make generate`. That's the whole mechanism: the file is named after the tag, the function is `Is` + the tag in PascalCase, and no flag has to point at anything — `pkg/validate` is the default, and a missing directory simply means no custom validators.

Two things worth knowing:

- Return `(bool, error)` instead of `bool` when the failure has detail worth keeping; the error is stashed in `ValidationError.Err` for the caller to unwrap.
- **A custom validator whose name collides with a built-in wins.** Your `email` rule replaces whisk's without forking anything.

## Escape hatches

Codegen covers the shape most DTOs have. Everything else has a hook, and reaching for one is normal — not an admission that the generator failed.

| When | Hatch |
|---|---|
| A rule tags can't express — cross-field logic, business state, a registry lookup | `func (s T) validate() error`, runs after the generated checks; its errors merge into the same chain |
| A field type whisk can't convert, a derived value, a default | `func (s *T) parse(src scan.Source) error`, runs last, after the generated binding |
| A body format that isn't JSON or XML | an `Unmarshal<Format>` function in `pkg/scan`, selected with `-format=<name>` |
| A tag that doesn't exist yet | `Is<Tag>` in `pkg/validate`, as above |

Both hooks are lowercase methods you write by hand; whisk detects them and calls them from the generated code.

## Without whisk

The generator is optional — this is the same handler with the codegen removed,
so the trade is visible rather than asserted.

**With whisk.** Two tags per field and one directive; `Into` runs the generated
`Parse`, then `Validate`:

```go
//go:generate go tool whisk $GOFILE

type createOrder struct {
    Currency string   `scan:"query=currency" validate:"required,currency"`
    Email    string   `json:"email"          validate:"required,email"`
    Items    []string `json:"items"          validate:"required,unique,dive,required"`
}

req, err := server.Into[createOrder](src)
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
        return nil, failure.BadRequest("malformed body")
    }

    out := &createOrder{Email: body.Email, Items: body.Items}
    out.Currency = src.Lookup("query", "currency")

    if out.Currency == "" {
        return nil, failure.BadRequest("currency is required")
    }
    if !validate.IsCurrency(out.Currency) {
        return nil, failure.BadRequest("currency is not a currency")
    }
    if out.Email == "" {
        return nil, failure.BadRequest("email is required")
    }
    if !validate.IsEmail(out.Email) {
        return nil, failure.BadRequest("email is not an email")
    }
    if len(out.Items) == 0 {
        return nil, failure.BadRequest("items is required")
    }
    seen := make(map[string]bool, len(out.Items))
    for _, item := range out.Items {
        if item == "" {
            return nil, failure.BadRequest("items contains an empty value")
        }
        if seen[item] {
            return nil, failure.BadRequest("items must be unique")
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

## Where this shows up

| Context | Page |
|---|---|
| HTTP handlers and request DTOs | [tools/http-handler.md](../tools/http-handler.md) |
| Queue consumers | [tools/scribus.md](../tools/scribus.md) |
| The full tag reference — every `scan` and `validate` rule, `dive`, `embed`, error shapes | the `README.md` of `gitlab.com/gorib/whisk` |
