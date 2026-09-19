# DTO Conversion

[Docs](../README.md) · Conventions

Each transport (gRPC, AMQP/scribus, HTTP) converts between external and
domain types. Two strategies exist, selected by the `mapper` key in
`waffle.yaml`:

- **`goverter` (default)** — a `convert` package with a `Convert` interface that
  [goverter](https://github.com/jmattheis/goverter) turns into compile-time
  conversion functions. It is the default because it **refuses to generate** on
  a field it cannot map: add a field to a model and generation fails until the
  conversion accounts for it. Scaffold a one-off converter anywhere with
  `waffle make:converter <path>`.
- **`dto`** — hand-written conversion functions in a `dto.go` file inside the
  handler package itself (`domain/<x>/cmd/grpc/dto.go`). No code generation, no
  extra package; pure functions, no logic, no errors. The cost is the check
  above: a hand-written mapper compiles whatever it leaves out, so a new model
  field silently stops reaching the wire. See
  [tools/grpc-handler.md](../tools/grpc-handler.md#dtogo--proto--domain-conversions)
  for the shape.

The rest of this page describes the goverter strategy:

```
domain/<x>/
    cmd/
        grpc/convert/
            convert.go               ← interface declaration with directives
            convert_goverter_gen.go  ← generated, do not edit
        scribus/convert/
            convert.go
            convert_goverter_gen.go
```

The framework helpers (`gitlab.com/gorib/convert/temporal`, `numeric`, `proto`)
provide ~40 conversion functions used across all services. Listing them as
`goverter:extend` in every `convert.go` (a) duplicates 30+ lines per file and
(b) hides project-level choices (e.g. "we use RFC3339, not RFC3339Nano").

## Pattern: project-wide whitelist

Create a single `pkg/goverter/convert.go` in your service that re-exports the
helpers your project blesses, then reference it from every domain
`convert.go` via one wildcard extend.

### `pkg/goverter/convert.go`

```go
// Whitelist of conversion helpers available to every domain Convert.
package goverter

import (
    "gitlab.com/gorib/convert/numeric"
    "gitlab.com/gorib/convert/proto"
    "gitlab.com/gorib/convert/temporal"
)

// Temporal — project default is RFC3339 (no nanoseconds).
var (
    StringToTime       = temporal.StringToTime
    TimeToString       = temporal.TimeToString
    StringPtrToTimePtr = temporal.StringPtrToTimePtr
    TimePtrToStringPtr = temporal.TimePtrToStringPtr
    // ...
)

// Numeric
var (
    IntToInt64       = numeric.IntToInt64
    Int64ToInt       = numeric.Int64ToInt
    IntPtrToInt64Ptr = numeric.IntPtrToInt64Ptr
    // ...
)

// Protobuf well-known types
var (
    TimestampToTime = proto.TimestampToTime
    TimeToTimestamp = proto.TimeToTimestamp
    // ...
)

// Project-specific helpers
var (
    // MoneyToString = money.MoneyToString
    // StringToMoney = money.StringToMoney
)
```

Each `var X = pkg.Func` re-export is recognized by goverter as a normal
conversion function — the wildcard extend below collects them all.

### Domain `convert.go`

A single extend line replaces the previous 7–15 (the package name is the
last path segment — `convert` under `cmd/<transport>/convert`, `dto` in
legacy or adapter locations):

```go
package convert

//go:generate go tool goverter gen ./...

// goverter:converter
// goverter:output:file ./convert_goverter_gen.go
// goverter:ignoreUnexported
// goverter:output:format function
// goverter:extend mymodule/pkg/goverter:.*
type Convert interface {
    BackInStockToProto(s *model.BackInStock) *alerts.BackInStock
}
```

### How `:.*` works

`goverter:extend pkg:Name` treats `Name` as a Go regular expression. When it
contains regex metacharacters (`.`, `*`, etc.) goverter scans the package
scope and registers every matching exported symbol whose type satisfies the
conversion-function signature `func(In) Out` or `func(In) (Out, error)`.
`:.*` matches everything exported in `pkg/goverter`.

## Overrides

### Per-converter (whole Convert)

Add another extend line below the wildcard. Goverter applies extends in
order and "last registered wins" when signatures overlap:

```go
// goverter:extend mymodule/pkg/goverter:.*
// goverter:extend gitlab.com/gorib/convert/temporal:TimeToStringNano
type Convert interface {
    // every time.Time → string in this Convert uses Nano
}
```

### Per-field

`map [SOURCE] TARGET | [PACKAGE:]FUNC` is stock goverter, not a gorib addition —
see [the `map` reference](https://goverter.jmattheis.de/reference/map). The
package and the function are separated by a **colon**; a dotted selector
(`temporal.TimeToStringNano`) is not resolved and fails with
`"temporal.TimeToStringNano" does not exist in package …`. Omit the package to
name a function declared in the converter's own package.

```go
type Convert interface {
    // goverter:map CreatedAt CreatedAt | gitlab.com/gorib/convert/temporal:TimeToStringNano
    Foo(*X) *Y
}
```

This overrides conversion for a single field without changing the whitelist.

## Adding a project-wide helper

1. Add the var to `pkg/goverter/convert.go`:
   ```go
   var MoneyToString = money.MoneyToString
   ```
2. Run `make generate`.
3. Every domain Convert that needs `Money → string` picks it up automatically.

## CLI scaffolding

The pattern is supported by waffle commands:

| Command | What it scaffolds |
|---|---|
| `waffle make:domain:grpc <domain>` | with `mapper: goverter` (default) — `domain/<x>/cmd/grpc/convert/convert.go` + ensures `pkg/goverter/convert.go`; with `mapper: dto` — nothing extra |
| `waffle make:domain:http <domain>` | with `mapper: goverter` (default) — `domain/<x>/cmd/http/convert/convert.go` + ensures `pkg/goverter/convert.go`; with `mapper: dto` — nothing extra |
| `waffle make:domain:scribus --driver <id> [--batch] <domain> <event>` | with `mapper: goverter` (default) — `domain/<x>/cmd/scribus/convert/convert.go` + ensures `pkg/goverter/convert.go`; with `mapper: dto` — nothing extra |
| `waffle make:converter <path>` | `<path>/convert.go` + ensures `pkg/goverter/convert.go` |

`make:converter <path>` covers the places no transport command reaches: it adds
a goverter converter at any directory — an outbound adapter, a client, a single
handler in a service that opted into `mapper: dto`. Example:

```
waffle make:converter domain/users/cmd/grpc/convert
waffle make:converter domain/orders/adapter/external-api/dto
waffle make:converter domain/users/adapter/auth-client/dto
```

The CLI uses the last path segment as the Go package name (`convert`,
`dto`, etc.). All commands are idempotent — running them on an existing
file is a no-op.

`make:domain:entity` (postgres) does *not* scaffold a goverter convert
— the sql adapter uses a hand-written `stored<X>` struct with `Unwrap()`
and `Inserts()` methods instead. See
[make:domain](../cli/make-domain.md#postgres-adapter) for that convention if you
need to mix the two.

## Why not interface embedding?

[Goverter](https://github.com/jmattheis/goverter)'s parser rejects unnamed
interface members — an embedded interface is one, so it has no method name to
report against. A common interface holding the extends, embedded into each
domain's `Convert`, fails with `method must have one name`. The
whitelist-via-vars pattern above is the intended way to share extends.

## Why not `-g extend …` CLI flag?

Goverter accepts `-g/-global` settings, but once a global extend has been
added per-converter `goverter:output:*` directives become an error
(`Cannot change output:* after extend functions have been added`). The
whitelist pattern avoids the conflict because all extends live in source.

## Choosing what goes in the whitelist

- **Always** include conversions used by ≥2 domains in the service.
- **Curate** when multiple framework helpers have the same signature
  (e.g. `TimeToString` vs `TimeToStringNano`). Pick the one that matches
  your project's transport contract.
- **Omit** helpers your project doesn't use — keeps the file readable and
  documents the policy.
