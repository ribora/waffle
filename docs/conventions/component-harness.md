# Component harness

[Docs](../README.md) · Conventions

A component test boots the **real container** over **real infrastructure** and replaces only the leaves. It is the executable form of the specification Rule #11 asks for: a caller drives the service the way production does — a proto request over a real connection, an event through the real consumer, a cron command as the app runs it — down through the real management and the real repositories.

`waffle tests:harness init` writes the files, and each one carries the reason for its own lines. So this page is what those comments cannot hold: the decisions nobody can make for you, the mechanisms behind the generated code, and what goes wrong at run time.

## Scaffolding it

`waffle tests:harness` reports which of these files are there; `init` writes the rest and leaves an existing one alone.

| file | what it holds |
|---|---|
| `tests/internal/harness/resources.go` | the resource list, `Up`/`Reset`/`Down`, `Env` |
| `tests/internal/harness/boot.go` | `App` and `Boot` — the cut that is a condition of booting |
| `tests/internal/harness/serve.go` | `Serve` and `Clients`, only if the container serves a transport |
| `tests/suite_test.go` | `TestMain` — `Env` before, `Down` after |
| `tests/boot_test.go` | the boot smoke test, written before any case |
| `config/diogen/replacements_test.go` | `replacements.Check()` as an ordinary test |
| `config/diogen/boundaries_test.go` | `boundaries.Check()` as an ordinary test |

Go beside Go in `tests/`, and `tests/feature/` left for the `.feature` files alone — only one of the two is a specification, and it is read by someone who does not read the other. The registry and the cut are separate files because they change for different reasons: one when a system appears, one when a seam does.

From a scenario the whole thing is two calls:

```go
app, err := harness.Boot(ctx, diogen.Replacements{ /* the cut */ })
clients, err := app.Serve()
```

What goes into the files is read off the generated container: a `gitlab.com/gorib/sql.Db` seam means postgres and every `domain/*/cmd/migrate` it finds, a `gitlab.com/gorib/redis.Db` seam means redis; `NewGrpcCommand` and `NewHttpCommand` decide the transports and hand over the handler, interceptor and router lists the daemon itself uses; every `grpc.ClientConnInterface` seam becomes a `grpcClient.Dead(...)` line.

No `--with=postgres,elastic`, and there never will be: a flag closes the set, and elastic 7 against elastic 8 is not even a different system, just a different class for the same one. What the container cannot say — a second resource, extra environment, which typed client belongs to which handler — is left as a named gap.

## Three layers, and only two are shared

| Layer | Where | Why there |
|---|---|---|
| the seam — `Resource`, `Set` | `gitlab.com/gorib/waffle/resource` | form, not a list: it imports no client, so it is tied to no version of anything |
| the implementations | `<pkg>/<pkg>test` — [`sql/sqltest`](#postgres), [`redis/redistest`](#redis), [`elastic/elastictest`](#elasticsearch) | they move with what they describe, in the same commit and the same version |
| `Boot`, `Env`, the resource list | the service, in `tests/internal/harness/` | `Boot` imports the service's generated `config/diogen`, and a library cannot depend on a service |

The third row is the reason there is no catalogue of systems in `waffle.yaml`. `init` writes the third row's files, but it never holds the list. **The registry is a literal in the service's own code:**

```go
var resources = resource.New(postgres, kafkatest.New(...))
```

Typed, checked by the compiler, failing at build time. Adding a system is editing that line, and waffle needs no change for it; swapping the whole stack — for a core team's own implementations, say — is swapping the import block, and that line does not change. waffle knows the contract, never the list.

## The seam

```go
type Resource interface {
	Provision(ctx context.Context) error // it is there and reachable, or a loud error
	Migrate(ctx context.Context) error   // schema, index mapping or topic set up to date
	Reset(ctx context.Context) error     // empty it, between scenarios
}
```

Three methods, and two optional halves discovered by assertion: `Named` (its name is quoted in errors) and `Ephemeral` (`Down(ctx)`, for a resource with something to remove when the run ends).

`Set` runs them: `Up` provisions and migrates **once per run** and memoizes the failure, so a hundred scenarios report the first diagnosis instead of hammering an absent server. `Reset` calls `Up` first, so a suite that only ever calls `Reset` is still correct. `Down` tears down in reverse and does not stop at the first failure.

**`Down` ends the run.** The set stays "up" afterwards — the `sync.Once` is not reset — so a `Reset` or a `Boot` after it fails rather than quietly reprovisioning. That is why `Down` belongs in `TestMain` and nowhere else.

**The resource never names the client type**, in either direction. How it reaches the system is its constructor's business, and the choice follows one question: is management part of the client the service already has?

## Postgres

`gitlab.com/gorib/sql/sqltest` works from a DSN, because `gorib/sql` exposes neither `create database` nor migrations. `init` writes the `sqltest.New(...)` literal and the goose call under it.

**Migration is a function you pass**, not something driven from inside. That keeps goose — or atlas, or tern, or hand-written statements — out of this module's dependency graph, which every service carries.

### The address chain

Explicit `Dsn`, then `Env` in order, then `Docker`. The container is last so that CI and anyone with their own server never start one.

**There is no default DSN.** A default carrying a password would put a credential in the repository, so an address that resolves to nothing is a loud error naming the variable to set. Error messages report host, port and database — never the DSN, because they end up in logs and CI output.

### The container

`Docker(image)` starts the image with `POSTGRES_HOST_AUTH_METHOD=trust`, so **no password exists** — which is what makes a run with no configuration at all possible. The runtime is invoked as a binary (`docker`, else `podman`, or pin one with `Runtime`), not through a library, so nothing lands in the `sql` module's dependency graph.

The container is named and **reused**, not removed: every service in a repository shares the default name and port, two services being two databases in one instance. Measured on a laptop:

| | cost |
|---|---|
| starting the container, to accepting connections | ~2.6s |
| creating the database and applying every migration | ~0.2s — inside the run-to-run noise |

That asymmetry is the whole design. Reuse the container, throw the database away:

```
docker rm -f gorib-harness-pg   # to reset the container
```

### Ephemeral database

`Ephemeral()` makes the database this run's own: the name gets the process id appended, `Down` drops it, and `Provision` sweeps up what earlier runs left behind. A schema left by a migration that has since changed can then never be inherited, and it costs about what the noise between two runs costs.

**The suffix is what lets two processes run at once**, which is not an exotic case: `go test ./...` runs packages in parallel, so a service with two harness-using test packages would otherwise have one process drop the database out from under the other.

It also means the name in a DSN is not the name that gets used — `warehouse_test` becomes `warehouse_test_48213`. That is the trade, and it is why the suffix is tied to this option and not to the address: `Ephemeral` is the caller saying the database is theirs to create and destroy. Without it the name is taken exactly as given.

**The sweep is what keeps the suffix from leaking.** `Down` runs on a normal exit; it does not run on Ctrl+C, a test timeout, a panic in `TestMain` or a cancelled CI job — and with a per-process name every such run would leave another database behind, forever. So `Provision` drops the leftovers, and a database has to satisfy all three conditions to be taken:

| Condition | Why |
|---|---|
| the name is `<base>_<digits>` | only a name this package could have produced; matched in Go, not with `LIKE`, so a base containing underscores cannot widen it |
| its comment is this package's mark | the mark carries the creation time — `pg_database` records none — so a database somebody else documented is never a candidate |
| nothing is connected **and** the mark is over an hour old | the drop is forced, so a single condition would be enough to kill a run that is merely slow |

`Down` must be called **once per process**, from `TestMain` — the database is shared by every test in the package, and dropping it between two of them would leave the second without a schema. That is what `tests/suite_test.go` is, and it is the only place `Down` belongs.

The drop uses `with (force)` — **postgres 13 or later**: our own pool is closed first, but a leaked connection would otherwise leave the schema behind.

### Reset

`Reset` truncates with `restart identity cascade`. The tables are **discovered from the schema** after migration — excluding a migration tool's own bookkeeping — so there is no list to keep in step with the migrations. `Tables(...)` pins them if you need to.

Truncation, not a database per scenario, and not a pinned transaction: the scenario talks to the service over a real connection, and context values do not cross that boundary.

## Redis

`gitlab.com/gorib/redis/redistest` works from a `host:port`, and the address chain is the postgres one: explicit `Addr`, then `Env` in order, then `Docker`, with the container last so that CI never starts one. `Reset` is `FLUSHDB`; `Migrate` does nothing, because keys carry their own shape.

**The container is this run's own, and `Down` removes it** — the opposite of postgres, from the same reasoning. Measured on a laptop:

| | postgres | redis |
|---|---|---|
| starting the container, to accepting connections | ~2.6 s | ~0.18 s |
| emptying it | ~0.2 s (create + migrate) | <1 ms (`FLUSHDB`) |

Postgres is a container that costs thirteen times what its contents cost, so you keep the container and throw the database away. Redis has no expensive contents to throw away, and the container itself is inside the noise — so you throw the container away instead. **Reuse what is expensive to build; discard what is cheap.** Copying the shape of `sqltest` here would have been copying its conclusion instead of its reasoning.

Two things follow. The runtime picks the port (`-p 127.0.0.1::6379`, read back with `docker port`) and there is no container name, because a fixed name and a fixed port are exactly how two runs would land in one instance — so `redistest` is parallel-safe by construction, and `Index` matters only for a server that was given rather than started. And `Down` runs `context.WithoutCancel`: a cancelled run is precisely when the container has to go, so teardown must not inherit the cancellation that triggered it.

**The sweep is what keeps a killed run from piling up.** `Down` does not run on Ctrl+C, a test timeout or a cancelled CI job. So `Provision` removes containers carrying the `gorib-harness=redis` label that are over an hour old and report no connections but its own probe. Unlike a postgres database, this needs no mark of its own: docker records the creation time.

## Elasticsearch

`gitlab.com/gorib/elastic/elastictest` goes over HTTP instead of through `elastic.Client`, for one concrete reason: emptying an index and making a seed visible both need `?refresh`, and no client here exposes it.

```go
var search = elastictest.New(
	elastictest.Env("CATALOG_TEST_ES"),
	elastictest.Prefix("test_"),
	elastictest.Index("product", productMappings),
)
```

Going over the wire also makes it blind to the client's major version — the management API for create, delete and refresh is the same on 7 and 8 — so a service on a back-ported v7 client uses it unchanged.

`Migrate` creates missing indices and **leaves existing ones alone**: a mapping change is a migration of its own, and guessing at one silently is worse than failing on the first query. `Reset` deletes by query with `refresh=true&conflicts=proceed`, keeping the mapping. `Refresh(ctx)` is exposed for a fixture that seeds through `Bulk` and then searches — without it that scenario flickers.

`Prefix` has to match what the container passes to `elastic.New`, or the resource manages indices the service never reads.

**No `Down`.** The indices `Migrate` creates are kept between runs, the way a postgres database is when `Ephemeral` is not asked for. Nothing removes them; drop the index by hand after a mapping change.

> Tested against `httptest`, not yet against a live cluster.

## Writing a resource

For a client that is not in gorib — your own Kafka wrapper, an in-house system — the harness is three methods next to that client:

```go
package kafkatest

// Declared here, at the consumer: your admin client already satisfies it.
type Admin interface {
	TopicExists(ctx context.Context, topic string) (bool, error)
	CreateTopic(ctx context.Context, topic string, partitions int) error
	DeleteTopic(ctx context.Context, topic string) error
}

func New(admin Admin, topics ...string) *Kafka
```

Where to put it, by number of consumers — with no exception for gorib:

| Consumers | Place |
|---|---|
| one service | `tests/internal/harness/kafkatest` |
| two or more | next to the client: client in `pkg/kafka` → `pkg/kafka/kafkatest` |
| the client moved into gorib | the harness moves with it |

**Measure before choosing what to keep between runs.** Both existing resources start a container, and they dispose of it oppositely — see [Redis](#redis) for the two numbers that decide it. Take the reasoning from whichever one your system resembles, not the shape.

## Choosing where to cut

The decision is written down, in `waffle.yaml`, as a permission list:

```yaml
replacements:
  allow:
    example.com/platform/api/contract: [ContractServiceClient]
```

What it does not name may not be replaced, in any form.
`TestReplacementsStayInsideTheList`, scaffolded beside the container by `waffle
tests:harness init`, reads the `Replacements` literals in the tests and fails
naming the field and the interface — so a cut nobody granted stops being a
question of whether a reviewer noticed, and shows up in `go test ./...` rather
than only when someone runs a command. `waffle tests:replacements check` is the
same comparison on demand. Two things it deliberately does not
decide: **which form** the replacement takes — mock, spy, fake or no-op — which
follows from the assertion and belongs to whoever writes the test, and anything
in `tests/internal/`, which is the same person's harness as the list itself.

A grant is keyed by interface, not by field, so an interface serving two
container fields is granted for both. `google.golang.org/grpc:
[ClientConnInterface]` therefore grants every connection in the container at
once, which is a reason not to grant it.

A **composite** field — `interface{ A; B }`, produced when registrations share
one container field — is granted only when every constituent is, because
replacing the field replaces every contract it merges. Those are the most
informative lines in the report rather than noise: a composite names the
contracts one management object owes, and each constituent is a caller, which is
a place a scenario binds.

`waffle tests:replacements` prints the menu; a person picks from
it, and what they pick goes into `replacements.allow`. The `# leaf` marker means
one thing only — *this constructor takes nothing else out of the container* — and
that is a heuristic, not the answer. It separates the two kinds of test, though:
a **component test** replaces leaves and lets everything else run for real,
while replacing an interior seam deletes the subtree below it — the code the test
was meant to exercise. A **unit test** builds its subject by hand, so
`Replacements` plays no part and the list has nothing to say about it.

**Cut above anything that speaks a language the code itself generates.** A
stand-in for such a level would have to speak it too, and then the test checks
the language instead of the behaviour. `sql.Db` carries SQL the repository
builds; `grpc.ClientConnInterface.Invoke(ctx, method string, args, reply any)`
carries the wire — a method name as a string and an untyped payload; an
elasticsearch client carries the query DSL.

So for an outbound gRPC dependency the cut is the **typed client**, one level
above the transport, even though `tests:replacements` does not mark it `# leaf` — its
constructor does take the connection out of the container, which is exactly why
it is not a leaf. Substitute both levels, for two different reasons: the typed
client because that is the surface worth faking, and the connection with
`grpcClient.Dead` so the boot stops reading the neighbour's host out of the
environment — and that second line is one `init` already wrote. This shape
recurs wherever a service adapts one transport to another — an http-to-grpc
adapter looks the same.

### What the marker cannot know

- **`sql.Db` is a leaf and must not be replaced.** The component test hands it a
  real database, so it stays out of `replacements.allow` and the harness supplies
  it instead.
- **An interior node can be the right cut**, as above. Not being a leaf is not a
  reason to leave it alone.
- **A boundary can sit below the leaf and outside the container.** A factory that
  builds its client per call from runtime data keeps the real boundary out of
  `Replacements` entirely: nothing supplied there can reach it, so the component
  test cuts at the factory's own port and the boundary below is mocked only for a
  unit test of the factory — which needs no grant, because `Replacements` plays
  no part in it.
- **An interface with no methods is not a seam.** Anything satisfies it, so
  granting it grants nothing; if one exists in the code, somebody intended a seam
  and left a stub.

## Replacing the leaves

`Boot` fills the cut: everything that is a condition of booting rather than a choice the test makes belongs in `boot.go`, not in the scenario. It supplies the database, installs the recorders and hands them back on `App` — a recorder the scenario had to construct would be a seam the scenario had to be granted, for something the harness owns either way.

`Dead` is the one line worth reading twice, because it is what people undo. **`init` writes it for you** — one line per `grpc.ClientConnInterface` seam it finds — so this is a line to leave alone rather than a stub to build.

It is there because **replacing a seam does not prune what feeds it**: `InitAppWith` is a flat eager sequence that skips a node only if that node itself was supplied, so the connection under a replaced typed client is still built and still reads its neighbour's host out of the environment. Reaching a dead connection panics, which says "the replacement did not take" louder than a silent call to a real neighbour.

**Guard every fill with `if cut.X == nil`.** That is the seam by which a scenario overrides what `Boot` installs — a live fake instead of a dead connection, a hand-built double instead of the harness recorder — without `Boot` having to know which scenarios want one. Assigning unconditionally takes that away, and the scenario's own `Replacements` literal is silently discarded.

One consequence of `di.For` worth expecting: it groups the consumers that share a connection, so the container has one field per *connection* rather than one per client, and that field is named after the first consumer of it. Four `Dead` lines can cover eight typed clients.

### `Noop()` is a recorder nobody queries

That is the rule that decides which to use. A silent stand-in is fine where the scenario asserts nothing about that leaf — and dangerous where it does. A silent consumer swallows `Apply` and `Bind`, so a typo in a handler's subject stays green forever while production receives nothing.

The scaffold therefore writes the recording one and leaves the choice on one line: whether anything is ever asserted about the logger is not knowable at `init` time, and a recorder nobody queries costs a slice, while a `Noop()` somebody needed costs a debugging session.

| Interface | Silent | Recording |
|---|---|---|
| `pry.Logger` | `pry.Noop()` | `pry.Recorder()` — levels, messages, fields, errors |
| `scribus.Consumer` | a recorder you do not query | `scribus.Recorder()` — wiring, plus `Deliver` |
| an amqp publisher | — | `amqp.NewPublisherRecorder()` — keys and payloads, `FailWith` |
| `outbox.Sender` | — | `outbox.Recorder()` — subjects and payloads, `FailWith`. Replace this and not the `Store`, so `Enqueue` still writes a row ([details](../tools/outbox.md#testing)) |
| `grpc.ClientConnInterface` | `grpcClient.Dead(name)` | — |

**`pry.Recorder()` is a driver, not a second logger.** It sits underneath the real logger, so a scenario asserts on records the production code built rather than on a parallel implementation of the same options.

### A mock is a declaration, a spy is an observation

Which one a seam gets follows from where the assertion sits, and that is decided by the notation rather than by taste.

A generated mock declares its expectation **before** the call. A specification written in Gherkin puts its assertions **after** the action — `Then` follows `When`, by grammar. So in a scenario suite every conversion to a mock ends in one of three places: the declaration moves into the `When` step and the `Then` line stops asserting anything; a new `Given` states the expectation, and the specification describes a call sequence instead of behaviour; or `.AnyTimes()` with a `.DoAndReturn()` writes into the very slices a spy would keep, plus a controller on top.

**So: in a godog suite a recorder is the default and a mock is the exception. In a unit test it is the other way round** — there the assertions are declarations already, the object under test is built by hand, and `.Times(0)` and `.Times(1)` are the clearest form there is.

Two things this does not excuse:

- A recorder must still assert the count. `.Times(1)` comes free with a mock; a spy that finds *a* matching call and stops will pass an implementation that calls twice.
- The lifecycle is not the obstacle it looks like. godog runs each scenario as a subtest when `Options.TestingT` is set, and `godog.T(ctx)` satisfies gomock's reporter, so a per-scenario controller with correct attribution is available. The grammar is the reason, not the plumbing.

**Waffle generates no mocks**, and no command will. The test that needs one
carries its own directive, which keeps the mock beside its only user and inside
`go generate ./...`:

```go
//go:generate go tool mockgen -typed -package security -self_package <this package> -destination client_mock_test.go <this package> Client
```

A `_test.go` destination inside the package under test is what makes a private
field reachable, and it keeps a single-use mock out of the production build and
out of everyone else's import graph.

### Delivering an event

`scribus.Recorder()` keeps the wiring the command asked for and delivers through **the handler the container installed** — so a scenario exercises the subject dispatch and the middlewares, not a bare `Run`:

```go
delivery, err := harness.Delivery("contracts.updated", payload)
if err != nil {
    return err
}
err = w.app.Consumer.Deliver(ctx, delivery)   // nil settled, error requeued
```

`amqp.Delivery{...}.Source(ctx)` builds the message. It exists because the real constructor is unexported: without it every suite writes its own `Source`, and each one gets the lookup scopes slightly differently.

## The in-process server

`Serve` is a method on `App`, and it answers with one `Clients` however many transports the service serves — a service can serve both, and a file per transport could not coexist: each would declare `Clients` and `Serve` in the same package. What `init` leaves for you is the typed clients: `proto.NewFacilityServiceClient(grpc.Conn())` per handler, because which client belongs to which handler is not in the container.

`waffleGrpc.Serve` runs the handlers over `bufconn` and chains the interceptors with **`UnaryChain`, the exact function the daemon uses**. That is the point of it being exported: interceptor order has caused a real production bug (see [tools/grpc](../tools/grpc.md)), and a harness that sorted its own copy would be checking its copy of the order rather than the service's.

The **lists** come from the container for the same reason — `a.ServerInterceptors()` and its siblings, and the router collection beside them, not the members written out one by one. A registration added to the chain later reaches the daemon and the suite together; written out, it would reach only the daemon, and the scenarios would go on passing against a chain that no longer ships.

`waffleHttp.Serve` is the same idea over `httptest`. It returns a `URL` and a `Client` — an ordinary `*http.Client`, so a scenario writes an ordinary request. The routers and the global chain are the container's own, and `Setup` is the same function the daemon runs, so the routes, the collision check and the middleware under test are the ones that ship — including its two errors, which is why `Serve` returns one. Both `InProcess` types close in `Clients.Close`.

## Rules that came from being wrong

- **Infrastructure is a requirement, not an inconvenience.** If the database is unreachable the harness fails loudly. A suite that silently skips looks green.
- **`Given` writes fixtures through the container's own repositories**, never through the operation under test. A `Given` expressed as a `Create` couples scenarios to each other; going through the repository means a schema change breaks fixtures loudly.
- **`Then` reads back through the repository**, not from the operation's own response. Strictly stronger: it catches "returned right, persisted wrong", and it is the same step whether the operation returns an object or nothing.
- **A scenario for a known defect asserts the intended behaviour**, so it is red until the defect is fixed. The red count has to equal the defect count — arithmetic that catches a case written with the wrong polarity.
- **Check whether you need a clock seam before adding one.** Comparing `time.Now()` against a fixture's date means relative fixtures express every time-dependent case and do not rot. A clock seam added for a test is production code written before the specification was approved.

## Not there yet

- **Elasticsearch is not parallel-safe.** `sqltest.Ephemeral()` gives each process its own database and `redistest` its own container; `elastictest` has neither, so two processes sharing an elasticsearch would share its indices. A prefix per process is the same idea and the same few lines, once a service needs it.
- **Rabbit has no resource yet.** A `flushdb`-shaped afternoon when a service needs one.
