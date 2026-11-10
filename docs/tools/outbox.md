# Outbox Tool

[Docs](../README.md) · Tools

The outbox delivers an event exactly when the transaction that produced it
commits. It exists for the gap in this shape, which every service has somewhere:

```go
invoice, err = m.invoiceRepository.Save(ctx, invoice)   // its own transaction, commits
if err != nil { return nil, err }
err = m.publisher.Publish(ctx, "invoices.paid", event)  // separate, after
```

A process killed between those two lines loses the event and leaves no trace of
having done so.

## The mechanism, which is easy to get wrong

`Enqueue` is an INSERT. Nothing is sent, and no code runs at commit time.

```
BEGIN
  INSERT invoice
  INSERT outbox row        ← Enqueue returns here and never runs again
COMMIT                     ← the row becomes visible to other connections
```

The relay polls on its own connection. Before the commit it cannot see the row,
after it can, and on rollback the row never existed for anyone. That visibility
rule is the whole mechanism — there is no commit hook and no notification to find.

The full argument, the decisions behind the claim protocol and the at-least-once
caveat are in [the package README](../../tools/outbox/README.md).

## Setup

```sh
waffle make:outbox --driver amqp
```

The module itself names no broker: it holds the rows, the relay and the command,
and takes delivery from a driver beside it. The one that ships is
`config/tools/outbox/amqp`, over `gitlab.com/gorib/amqp`. The driver is a required
flag, not a default, for the same reason `make:domain:scribus` takes one — picking
a broker is the question the split exists to leave open. It refuses before writing
anything if `config/di/modules.go` has no line for the driver named.

That writes `domain/outbox/cmd/migrate` — the migration runner and the table —
registers it with `migrate.MigrationsTag`, and uncomments the driver:

```go
// config/di/modules.go
_ "gitlab.com/gorib/waffle/config/tools/outbox/amqp"
```

One line, because the driver blank-imports the module it drives. Enable the module
alone and nothing provides `outbox.Sender`, which diogen reports at generation time
rather than leaving for the first publish.

The table is written into your service rather than shipped by the framework, so
you can read it, index it further with `waffle make:domain:migration outbox …`,
or port it by hand to a database that is not PostgreSQL.

Two rules if you edit it. It has **no foreign keys into domain tables and no
dependency on their triggers**, because the sources collected through
`migrate.MigrationsTag` run in version order and its version says nothing about
theirs — an outbox that waits on another domain's migration can fail on a fresh
database. And `created_at` is
sub-second where the entity template is `timestamp(0)`: two events a millisecond
apart are two events.

Each index answers one query, so before dropping one:

| Index | Used by |
|---|---|
| `outbox_claimable` | the claim's first statement, and the backlog's oldest |
| `outbox_claim_token` | the claim's second, which reads the entry back by the token it just wrote |
| `outbox_sent_at` | the purge |

## It is not a PostgreSQL feature

Only the migration this ships is PostgreSQL. The reference `Store` is
dialect-neutral by construction: every statement goes through the builder, there
is no `FOR UPDATE SKIP LOCKED` and no raw SQL, and the claim reads its row back by
a token it wrote rather than by `RETURNING`, which MySQL does not have.

The suite that runs in this repository covers PostgreSQL. MySQL 8 and SQLite
(modernc) have been checked by hand, with the same `Store` and nothing but a
ported table — run `outboxtest.Suite` against your own port rather than trusting
that.

Porting the table is the whole job, and two things in it are easy to get wrong:

**MySQL** — `bytea` → `longblob`, `timestamp(6) with time zone` → `datetime(6)`,
and the partial indexes become plain ones (`(sent_at, id)` covers the claim).
`text` columns that are indexed need a length, so `varchar`.

**SQLite** — declare the time columns `timestamp`, not `text`, or the driver hands
back a string and the scan fails. Its `current_timestamp` also ticks once a
second, so entries enqueued in the same second share a `created_at`; delivery
order is unaffected, because the relay orders by `id` and that is a UUIDv7.

This needs a `gitlab.com/gorib/sql` new enough not to render a `RETURNING` clause
for a driver that has none — before that, every `Insert` and `Update` through
`sql.Repository` is rejected by MySQL.

`domain/outbox` holds a migration runner and nothing else. The outbox has no
business model; it lives under `domain/` so that the migration commands work on it
like they do on everything else.

The module pulls in [`config/service/sql`](../reference/running-locally.md) and
`config/tools/migrate`; the AMQP driver pulls in `config/service/amqp`, which is
where `RABBIT_DSN` and `RABBIT_EXCHANGE` are read — once, so a service that also
publishes directly does not end up with a second connection.

## Publishing through it

The publisher seam is your service's own interface, so widen it by one method:

```go
// pkg/publisher/publisher.go
type Publisher interface {
	Publish(ctx context.Context, subject string, document any) error
	Enqueue(ctx context.Context, subject string, document any) error
}
```

and point it at the module's instance, which needs no environment of its own:

```go
// config/di/di.go
di.Wire[publisher.Publisher](func(p outbox.Publisher) publisher.Publisher { return p })
```

Then change the call sites that must not lose their event:

```go
-  return m.publisher.Publish(ctx, "invoices.paid", event)
+  return m.publisher.Enqueue(ctx, "invoices.paid", event)
```

|           | atomic with the transaction | confirmed by the broker | survives a crash after the commit |
|---|---|---|---|
| `Publish` | no                          | yes                     | no                                |
| `Enqueue` | yes, when there is one      | yes, by the relay       | yes                               |

**A call site that wants the guarantee has to own its transaction.** `Enqueue`
joins the transaction the context carries and opens its own when there is none —
so a method whose repository commits on its own and then enqueues is atomic with
nothing. Wrap the two in one transaction, or the change and the event can still
part company.

A service that never wants the outbox enables `config/service/amqp` alone and
keeps a one-method interface; nothing about `Enqueue` reaches it. The adapter is
the same one line against the other type:

```go
di.Wire[publisher.Publisher](func(p amqp.Publisher) publisher.Publisher { return p })
```

## Running the relay

Nothing is delivered until the relay runs. It is a daemon, so it is its own
deployment:

```sh
./app outbox
```

Several replicas are safe — the claim is the only coordination and it is atomic.

```sh
./app outbox status
```

```
unsent	12
failed	1
oldest	3m21s
```

`oldest` is the number that matters: a dead relay, a refusing broker and a poison
entry all show up as it growing. It is `created_at` from the database subtracted
from the clock of the pod that ran the command, so clock skew between the two
lands in it — a negative age means they disagree, not that an entry is from the
future.

A subcommand rather than a gauge, deliberately. A gauge has to be published by
something, and the obvious something is the relay — which stops publishing exactly
when it dies, the failure you most want to hear about. `outbox status` answers from
any pod, at any time, whether or not a relay is alive. `Store.Backlog` is exported,
so a service that wants the number on a dashboard can publish it from wherever it
likes.

After `OUTBOX_MAX_ATTEMPTS` failures an entry stops being claimed but is kept, with
its `attempts` and `last_error`, for diagnosis:

```sh
./app outbox reset          # every entry that ran out of attempts
./app outbox reset <id>     # that one, whatever its attempts
```

An entry already delivered is never resurrected, by either form.

## Environment

| Variable | Default | What it does |
|---|---|---|
| `OUTBOX_MAX_ATTEMPTS` | `10` | Failures before an entry is left for `reset` |
| `OUTBOX_RELAY_NAME` | the hostname | Written into `claimed_by`, so a stuck claim names its pod |
| `OUTBOX_POLL_INTERVAL` | `5s` | The longest pause between passes when the outbox is empty |
| `OUTBOX_STALE_AFTER` | `1m` | How long a claim is honoured, and so how soon a failure is retried. Under a second is raised to a second — a claim stale on arrival is no claim, and two relays would take the same entry |
| `OUTBOX_RETENTION` | `168h` | How long delivered entries are kept before the relay purges them. Zero or less never purges, and the table grows without bound |
| `OUTBOX_PURGE_INTERVAL` | `1h` | How often the relay looks for delivered entries to purge |

`DB_DRIVER` and `DB_DSN` come with `config/service/sql`; `RABBIT_DSN` and
`RABBIT_EXCHANGE` with the AMQP driver.

## Replacing a piece

To deliver somewhere other than RabbitMQ, write a driver module: one
`di.Wire[outbox.Sender](…)` and whatever its broker needs. Enable it instead of
`config/tools/outbox/amqp` and nothing AMQP is linked, configured or required —
the module names no broker, so there is no default to fight.

The rest is `di.Fallback()`, so registering your own for the same slot drops the
default entirely:

| Slot | Default | Replace it to |
|---|---|---|
| `outbox.Store` | `store.NewStore` over `sql.Db` | hold the rows somewhere else — run [`outboxtest.Suite`](../../tools/outbox/outboxtest) against it |
| `outbox.Relay` | `outbox.NewRelay` | change how draining is paced |

`Sender.Send` must return an error when the broker has not confirmed delivery.
A Sender that reports success for a message the broker never took turns the
outbox into a machine for losing events with confidence — the relay would mark the
row sent on its word.

## Testing

Replace the Sender, not the Store: the scenario then runs against a real
outbox, so `Enqueue` writes a real row and the test can assert on the row.

```go
sender := outbox.Recorder()

container, err := diogen.InitAppWith(diogen.Replacements{OutboxSender: sender})
```

It keeps every `Send` — subject, payload, correlation id and trace — read back with
`Sent()` or `WithSubject(subject)`. `Publish` reaches it directly, and an enqueued
row once a relay runs. `FailWith(err)` makes every `Send` fail from then on, for a
scenario about a broker refusing, and `Reset()` clears both.

A recorder for the publisher instead would make "the event went out" and "the
event was lost" look the same, which is the thing being fixed.

## Next

| | |
|---|---|
| The claim protocol and the at-least-once caveat, in full | [the package README](../../tools/outbox/README.md) |
| `Sender`, `Store`, `Publisher`, `Relay` signatures | [reference/interfaces](../reference/interfaces.md#toolsoutbox-package) |
| Where the outbox sits in a modular monolith | [Recommended architectures](../architectures.md#events-leaving-the-binary) |
| Publishing without the outbox, straight to the broker | [tools/scribus](scribus.md) and `config/service/amqp` |
| What has to be running and in `.env` | [reference/running-locally](../reference/running-locally.md#by-module) |
