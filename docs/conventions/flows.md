# Flows

[Docs](../README.md) · Conventions

A flow is what happens after one way in: the route, command or message from outside, every service it reaches, what each sends on, and in which of its cases. Waffle writes flows from two records and never from prose, so a flow cannot go stale without a test failing:

| file | written from | kept fresh by |
|---|---|---|
| `tests/surface.yaml` | the service's code: what it publishes, consumes, serves and calls | `TestSurfaceMatchesTheCode`, in the service's own suite |
| `tests/flows.yaml` | the component suite: every input a case sent, and what the service sent on | `harness.Main`, after a whole green run |
| an index and one page per input, beside it | the two above, for every service below a root | `waffle flows check` |

The code says what *can* happen, the suite what the cases *make* happen. Each alone misleads: the code cannot tell a path that runs from one that cannot, and the suite is silent about what no case reaches. Read together, the difference between them is the list of gaps.

## The code's record

```
./waffle flows record
```

writes `tests/surface.yaml` from the module's code, and the harness scaffolds `tests/surface_test.go`, which fails when the file falls behind. What is recorded is what the libraries a service talks through receive as the name of a message:

| | recognised at |
|---|---|
| publishes | `outbox.Publisher.Publish`/`Enqueue`, `amqp.Publisher.Publish`, the `amqp091` channel's `Publish*` |
| consumes | `amqp` `Bind`, the `amqp091` channel's `QueueBind` |
| serves | `grpc` `RegisterService` (by the `_ServiceDesc` it is given), and every route of a `gitlab.com/gorib/http/server` mux under its group prefixes |
| calls | `grpc` `Invoke` and `NewStream` — a generated client's own calls — and requests through `gitlab.com/gorib/http/client`, a variable path segment written `{}`, a path with no constant part at all written `external` |
| domains | in a modular monolith, a call from `domain/A/adapter` to a `domain/B/service` type the container wires in: `rent → invoice: Invoices.Issue` |

A service's own interface in front of one of those — its `Publisher` over the outbox — counts as the library, and a wrapper function is followed to its callers. A name that is not a constant at the end of that chain is refused: it is a contract, and a contract has a name. The one exception is an HTTP request whose whole address comes from data — a supplier's file URL — which is recorded as `GET external` and lands outside the set.

## The suite's record

The generated harness traces everything that crosses the service's edge during the suite: the calls the suite sends to its server, the calls the service makes to what a scenario replaced, the events it publishes and consumes, and the commands a step runs. A godog suite calls

```go
func initialize(sc *godog.ScenarioContext) {
	harness.Steps(sc)
	// …
}
```

so that a case is known by its file as well as its title, and only its `When` counts as its input — a `Given` that calls the service sets up, a `Then` checks. Without it every call is an input.

`harness.Main` compares the trace with `tests/flows.yaml` after the suite, and fails the run on a difference. Only a whole green run is compared: a failing or narrowed one (`-run`, `-skip`, `-godog.tags`, `-godog.paths`) saw part of the service. To rewrite the file:

```
go test ./tests/... -args -flows.update
```

The file lists every input, and under each its **branches**: the cases that, having received it, sent the same messages on in the same order, or were refused the same way. A refusal is named by what a case asserts about it — `NotFound contract_not_found`, `HTTP 403 forbidden` — not by its message, which carries ids. A block sent again straight after itself is one pass of a loop, so the number of items a case had does not split its branch; a message sent again after others is a step of its own. A case in two branches — a scenario that received the input twice by different paths — is two cases.

`description` fields are the only thing written by hand. They are kept by anchor across rewrites; one whose input or branch is gone moves to `orphaned`, and the check fails until somebody deletes it or moves it.

## The project's flows

```
./waffle flows <root> <index.yaml>
./waffle flows check <root> <index.yaml>
```

finds every waffle service below `root` the way `go test ./...` finds packages, takes them as the whole system, and writes the index and the pages beside it; `check` writes nothing and fails on any difference. A flow starts at every way in: a route, a command, or a message no traced service sends. A monorepo keeps them in its own directory — `.docs/flows/` — and runs the command from its root; the index is the project's, not a service's.

**Pages.** One per flow, and one per input a flow reaches. A flow's page draws the sequence, each step's sends nested under the arrow that reached it and inside the condition — "in 2 of 3 cases of step 1" — that made them; an event a service consumes itself is marked. The page lists the cases of its own input with their comments from the `.feature` file; a step further on shows its branches and how many cases each has, and links to its own page, so a step many flows share — authentication, say — is written once.

**The index** holds, besides the flows:

| section | what it is |
|---|---|
| `links` | every message one traced service sends and another receives; `notes` names fields the receiver's cases send and the sender's never do |
| `untested` | a link the code has between two traced services and no case follows |
| `outside` | what a service sends that no traced service received: taken by nobody's case, taken only by a wildcard subscription, or out of the system |
| `system` | the records against each other, below |
| `orphaned` | a flow's `title` or `description` whose flow is gone |

`system` fails the command on an event published and consumed by nobody, subscribed to and published by nobody, an RPC called and served by nobody, served and called by nobody, or an HTTP request more than one service routes. It only reports an event taken by a wildcard alone, a service consuming its own event, and an HTTP request no service routes: each of those is a legitimate design.

## What is not there

- **A party outside waffle.** A service with no records is not in the set: an RPC only it calls is "served, nobody calls", and an HTTP request to it leaves the system — both true of the records, which is all the check reads. There is no way to declare a link by hand, in config or in code; a declared link is one more thing to forget.
- **Outgoing HTTP in a trace.** The trace sees the service's own server and the calls a scenario replaced; a request to another system goes through a client the harness does not wrap, so the pages show it only where the code's record names it.
- **Paths no case takes.** A branch in the code that no case reaches is in neither record. Coverage of the suite over the production packages is the tool for that.
