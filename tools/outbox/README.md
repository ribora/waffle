# outbox

A service commits a business change and then publishes:

```go
invoice, err = m.invoiceRepository.Save(ctx, invoice)   // its own transaction, commits
if err != nil { return nil, err }
err = m.publisher.Publish(ctx, "invoices.paid", event)  // separate, after
```

A process killed between those two lines loses the event, and nothing anywhere
records that it happened. The outbox closes that window by writing the event into
the same transaction as the business change, and delivering it afterwards from a
row that either committed with the change or never existed.

## The one thing to get right

**`Enqueue` is an INSERT. It is not a send, and no code of ours runs at commit
time.**

```
BEGIN
  INSERT invoice
  INSERT outbox row        ← Enqueue returns here and never runs again
  …anything, however long…
COMMIT                     ← the row simply becomes visible to other connections
```

There is no commit hook and no notification. The relay polls on **its own
connection**: before the commit it cannot see the row, after the commit it can,
and on rollback the row never existed for anyone. That visibility rule *is* the
mechanism. Do not build a callback on top of it, and do not look for one — there
is nothing to find.

Two consequences fall straight out of it:

- a long transaction delays delivery by its whole duration, because the row is
  invisible until it commits;
- the guarantee is monotonic. `Enqueue` outside a transaction opens one of its
  own, so the event is still delivered; inside a transaction it is additionally
  atomic with it. Delivery always, atomicity when you asked for it.

## Two methods, two guarantees

```go
type Publisher interface {
	Publish(ctx context.Context, subject string, document any) error
	Enqueue(ctx context.Context, subject string, document any) error
}
```

|           | atomic with the transaction | confirmed by the broker | survives a crash after the commit |
|---|---|---|---|
| `Publish` | no                          | yes                     | no                                |
| `Enqueue` | yes, when there is one      | yes, by the relay       | yes                               |

The guarantee a call site asked for is readable in its one line. That is the
reason for two names rather than a flag or an ambient mode: which of these you get
should never depend on what the caller happened to have on the context.

## The relay

`./app outbox` runs it: claim the oldest entry, send it, mark it sent. It is a
supervised goroutine the module owns, not a cron job — `tools/cron` refuses to
schedule a daemon, and a cron interval would become the delivery latency. It polls
with backoff: without pauses while there is work, rarely when there is none.

Delivery is **at-least-once**. A relay that sends an entry and dies before marking
it sent will send it again. Consumers must tolerate that; nothing here pretends
otherwise.

`LISTEN/NOTIFY` is out of scope on purpose. It is a postgres-only accelerator that
can never replace polling, because a notification sent while the relay is
reconnecting is simply lost.

## Health is one number

The age of the oldest unsent entry. A dead relay, a broker refusing everything and
a single entry nothing will ever accept all look the same from outside: that
number growing. It spans two clocks — `created_at` from the database, subtracted
from the clock of the pod that asked — so a negative age means those two disagree.

```sh
./app outbox status
```

```
unsent	12
failed	1
oldest	3m21s
```

A subcommand rather than a gauge, deliberately. A gauge has to be published by
something, and the obvious something is the relay — which stops publishing exactly
when it dies, the failure you most want to hear about. A stopped gauge reads as no
data, not as an alarm. `outbox status` answers from any pod, at any time, whether
or not a relay is alive.

`Store.Backlog` is exported, so a service that wants the number on a dashboard can
publish it from wherever it likes.

After `OUTBOX_MAX_ATTEMPTS` failures an entry stops being claimed but is **not**
deleted — `attempts` and `last_error` stay for whoever has to work out why.

```sh
./app outbox reset          # every entry that ran out of attempts
./app outbox reset <id>     # that one, whatever its attempts
```

## Writing a Store

The storage seam is `Store`, so an outbox can be backed by something other than
`gitlab.com/gorib/sql` — gorm, for instance. The requirement that matters cannot
be written into the interface:

> `Add` must join whatever transaction the context already carries.

An implementation that quietly takes its own connection satisfies every signature,
passes every method test, and destroys the guarantee in silence. So run the
conformance suite:

```go
outboxtest.Suite{
	Store: func(t *testing.T) outbox.Store { … },
	Begin: func(ctx context.Context) (context.Context, func() error, func() error, error) { … },
}.Run(t)
```

Its first case adds a row inside a transaction, rolls the transaction back, and
expects `Claim` never to see it. That case is the reason this package ships a
suite at all.

## Decisions worth not relitigating

**One row per transaction, not a batch.** A batch mixes unrelated events, so one
stuck entry blocks everything behind it and a partial failure needs awkward
partial-commit rules. These are business events; throughput is not the constraint.

**Claim is a conditional `UPDATE … WHERE claimed_at IS NULL`, not
`FOR UPDATE SKIP LOCKED`.** Atomicity comes from the UPDATE itself and works on
any dialect. `SKIP LOCKED` would raise the bar for writing a new driver and cannot
degrade safely: plain `FOR UPDATE` stalls the relay, and no lock at all publishes
duplicates.

**Nothing here is PostgreSQL-specific except the migration the scaffolder writes.**
The claim also reads its row back by a token it wrote rather than by `RETURNING`,
which MySQL does not have, and the candidate sits in a derived table because MySQL
will not read the table an `UPDATE` targets from a subquery of its own `WHERE`.
The suite in this repository covers PostgreSQL; MySQL 8 and SQLite were checked by
hand with this package unchanged. See [the docs page](../../docs/tools/outbox.md)
for what porting the table involves.

**`Failed` leaves the claim in place.** A crashed relay and a refused message
leave the row in the same state, so one timer — `OUTBOX_STALE_AFTER` — covers
both. Releasing the claim on failure would let one poison entry spin the loop.

**The relay is given the real Sender, never the `Publisher`.** Wiring the wrong
one would make the relay write rows instead of sending them, which is silent and
easy, so `Publisher` deliberately implements neither `Sender` nor
`amqp.Publisher`: the mistake does not compile.

**The table is self-contained** — no foreign keys into domain tables and no
dependency on their triggers, because the order of migration sources collected
through `migrate.MigrationsTag` is not guaranteed.

## What it does not do

- It does not make delivery exactly-once. See at-least-once, above.
- It does not warn when `Publish` is called inside a transaction, which is almost
  always a bug: the event goes out and a rollback cannot recall it. It is
  detectable at runtime through `sql.TransactionFromContext`, and nothing here
  does it yet.
- It does not order across subjects. Entries are claimed oldest-first by id,
  but one relay claims one entry at a time and several replicas interleave.
- It does not re-encode a row for a different driver. `Enqueue` encodes through the
  `Sender` and stores the bytes; the relay sends them as they are. Swapping the
  driver on a table that still has unsent rows hands the new one payloads the old
  one wrote — drain the outbox first.
