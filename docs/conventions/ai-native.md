# AI-native development

How a change gets made when a coding agent writes most of the code.

The premise is a shift in what is scarce. Writing code is no longer the
bottleneck; deciding what *correct* means is. So the human artifact of a change
stops being reviewed code and becomes an approved specification, and the agent's
job is to satisfy it without being able to move the goalposts.

`AGENTS.md` carries the enforceable form of this as **Rule #11**. This page is
the reasoning behind it, what the framework gives you, and — honestly — what is
not built yet.

## The cycle

```
requirements ──▶ specification + delta list ──▶ approval ──▶ steps ──▶ red-green
                          ▲                                              │
                          └──────────── conflict or constraint ──────────┘
```

1. **Requirements in, specification out.** The agent produces an executable
   specification and a *delta list*: every place the specification says something
   the source document does not, plus what was deliberately left out of scope.
2. **Verification, then approval.** A human reads the source document and the
   delta, and corrects or approves. No production code before approval.
3. **Steps and fixtures**, bound to the real container. Still no production code.
4. **Red-green**, changing production code only. The specification is frozen.

Anything that cannot be built without changing the specification goes back to
step 2 through the person who approved it. The agent never edits an approved
test on its own judgement — an agent told to make a test pass will otherwise
weaken the test, and that is the failure mode this whole shape exists to prevent.

There is a subtler version of that failure, and it took catching in practice: a
case written for a *known defect* must assert the behaviour you want, so it is
red until the fix lands. Write what the code currently does and the defect
becomes a green regression boundary — frozen, protected, and reading as
passing. The check that catches it is arithmetic: the number of red cases has to
equal the number of known defects, and a mismatch means either a defect got
frozen or a case is wrong.

### What the two failures were, and what actually fixed them

Rule #11 has been broken twice by agents that had it in front of them, and the
two are worth reading together because the second one refutes the obvious fix.

**First: crossing into the platform's zone.** A session told only "build the
scenarios per Rule #11" wrote the cases, went straight on to the harness and the
steps, and edited `waffle.yaml` to grant itself the nine seams its hand-written
fakes needed. The prohibition was in the file, in three places.

**Second: the right lane, the wrong order.** A later session stayed inside its
own zone and wrote production code across two domains before any case existed.
Its own diagnosis was exact: it had read the short mid-work imperatives that
settle a design question — "add it", "forbid it", "do B" — as authorisation to
implement. The rule said what a *task* is not; it never said what an *answer to
your own question* is, and every such answer resets an agent's sense of having
just been handed a job.

The expensive part was not the order. The change pinned nothing: the
specification said the same thing before and after, and the suite would have
been exactly as green without it. Worse, it could not be pinned at that cut —
telling the two designs apart needs a failure *inside* the transaction at the
declared-contract level, and no seam offers one. So the correct move had been to
stop and report a gap, which the rule does say.

**What did not work: more prose.** After the first failure the fix was a
paragraph of argument, and one line of it made things worse — "run the check
before you hand anything over" turns a red check into a list of the exact lines
that would make it green. What worked was a **review surface that does not
depend on the author's compliance**: `waffle.yaml` must never appear in the diff
of a specification task, which costs a reviewer one glance at the file list.
After the second failure the same surface was extended to production files and to
`CONTRIBUTING.md`, and the red-count check gained its mirror — every production
file in the diff is named by a case that went red.

The generalisation, now with two data points: **prose describing a state does not
stop an agent with momentum. A prohibition needs either a mechanism or a surface
a third party can check.** Both incidents were argued against, at length, in the
file the agent had read.

### Why a delta list rather than "read the scenarios"

Requirements written by analysts always have gaps, and an agent transforming them
into a specification must resolve those gaps to make each case unambiguous. It
will do so silently, because a case that is ambiguous is not executable.

So the thing worth reviewing is not "does the specification match the document" —
it is "where did the specification decide something the document did not". That
comparison is bounded: read the document, then confirm or correct a short list.
Diffing prose against twenty scenarios in your head is not bounded, and a
specification nobody reviewed carefully is worse than none, because approval has
made a guess authoritative.

The out-of-scope half of the list matters for a specific reason: a reviewer
comparing two documents reliably catches a wrong answer and reliably misses a
missing question. Absence has to be stated to be reviewable.

### Measurements the rule leans on

The instructions in `AGENTS.md` are terse where these numbers are the reason for
them.

- **One specification per declared contract, not per service.** On dungeon, a
  domain's management served four callers — gRPC, a consumer and two cron
  commands — with no overlap at all, so a specification written against the gRPC
  operations alone would have covered one contract in four. The
  `interface{ A; B; C }` entry that `waffle tests:replacements` prints
  for a shared container field *is* that list of contracts.
- **Adjudication of defects is the human's.** Deriving candidate defects from
  code is the cheap half and it is wrong often: across the two services this was
  built on, several proposed defects turned out to be deliberate business rules
  and one could not be reproduced at all.
- **The red-count check earns its place.** On both services, "the number of red
  cases equals the number of known defects" caught a real mistake — once a
  defect frozen as a green boundary, once a case that was simply wrong.
- **A case can fail to discriminate and still look complete.** Three cases
  pinning "the adapter is asked with the facility's own configuration" all used
  one facility with one provider, so an implementation holding a single
  configuration for the whole process passed all three. The second facility is
  what turned them into cases.
- **Hand-computed numbers fail at about one in a hundred.** On kostej, one row
  of a hundred and four had the wrong date — month-end clamping done in the
  head — and the code was right. Which is why the gate is told that the
  arithmetic was read rather than run.
- **An unexpected failure is as likely to be the step as the service.** One of
  sixteen on kostej: a column added to a fixture table that the step never read.

## Why this needs the container cut

A specification for a feature is stated at the boundary of the service — given
this state, when this call arrives, then this happens. Running it means the
service is assembled, and there are two ways to get there: the full environment,
or the service with its outside edges replaced.

In a fleet of two dozen services the first is rarely available, so the second is
what makes the cycle possible at all. The generated container already provides
it:

```go
app, err := diogen.InitAppWith(diogen.Replacements{ /* the cut */ })
```

Every field of `Replacements` is a seam — a place a test *may* fill in instead of
letting the container build it, if `replacements.allow` in `waffle.yaml` grants
it. See [DI overview](../di/overview.md#testing-with-the-container) for the
mechanics and
[tests:replacements](../cli/overview.md#testsreplacements) for the
tooling.

### Cut at the leaves

`waffle tests:replacements` marks a seam `# leaf` when its default
constructor takes nothing else out of the container. Those are the boundaries of
the process; everything else is a node inside it.

That distinction is the difference between the two kinds of test:

- a **component test** replaces leaves and lets the rest run for real. It is what
  a specification at the service boundary needs.
- replacing an interior seam instead deletes the subtree below it — which is the
  code the specification was meant to exercise. That is a unit test wearing a
  component test's clothes.

A **unit test** does not use `Replacements` at all: it builds the object under
test directly, so the permission list has nothing to say about it — its mock is
generated by a `//go:generate` line next to the test that needs one.

### Where the marker does not reach

The boundary of the container and the boundary of the process are not the same
thing, and they miss in both directions:

- reaching another service over gRPC, the leaf is `grpc.ClientConnInterface`,
  while the seam worth replacing is the generated `*ServiceClient` one hop above
  it. Mocking a connection means answering with encoded protobuf. Replace
  **both**: supplying the typed client does not stop the connection under it
  from being built, because `InitAppWith` is a flat eager sequence and prunes
  nothing — so the boot still reads that service's host and port. The
  connection is then unreachable, which makes a panicking stub the honest
  filling.
- behind a factory that builds its clients from runtime data, the real boundary
  is *below* the leaf and outside the container entirely. A mock of it cannot
  reach the app through `Replacements` at all — it serves a unit test of the
  factory, while the component test still cuts at the factory's own port.

A leaf is also not always worth a mock. `sql.Db` is one, but a component test
hands it a real connection to a test database. The rule behind that: a
dependency carrying a language the code itself generates cannot be stood in for,
because a stand-in has to understand the language, and then the test checks the
language instead of the behaviour. Same for a port whose production
implementation already has a null variant selected by data: nothing needs
replacing, so it is classified and left alone.

Within the cut, what gets a generated mock and what gets a no-op is structural,
not a matter of taste:

- a seam the test **programs** — an outbound client, an adapter — gets a mock.
- a seam that only has to be **non-nil for the boot** — a message consumer, an
  unreachable connection, the logger — gets a stand-in owned by the harness, not
  a mock (`grpcClient.Dead()` for a connection, a recorder for the rest).
  Declaring expectations for the calls a container makes while constructing
  itself turns "this does not matter" into work in every case.
- a seam a later assertion **reads back** gets a recorder — `pry.Recorder()`,
  `scribus.Recorder()`. The line between the last two is the specification: a
  no-op is a recorder nobody queries, so silence is right exactly where nothing
  is asserted, and hides the subject everywhere else.

And the negative assertion is written, never implied: `.Times(0)` rather than
leaving the call undeclared and letting the controller fail. The suite is a
document that someone approves, and an assertion that exists only as a missing
line cannot be reviewed.

None of this is derivable, which is why the marker is a fact about the graph and
not a recommendation, and why `replacements.allow` is curated by a person.

## What it costs

**Not the thinking.** If you already read incoming requirements, fit them into
the project, and find the contradictions before writing code, this cycle adds no
new step — it externalizes one you were doing in your head. The delta is the cost
of writing it down.

**Writing it down buys two things** that the in-your-head version cannot: an
agent can work against it unsupervised, and a second person can do the
verification later. It is also what onboarding cannot otherwise transfer — a new
joiner reading code infers intent, while a specification shows which decisions
were deliberate.

The two artifacts do not end up in the same place, and should not. The
specification is about the code, is read as current, and belongs in the
repository — with the reasoning for each case written into it, which is the part
a new joiner needs. The delta list is about the *requirement*: a record of what
the document left open and what was decided, addressed to whoever wrote it. It
belongs with the ticket. Committing it would put a negotiation that was finished
at approval into a place where everything is read as still true.

**The ceiling moves to verification.** Team output becomes bounded by how many
specifications one person can carefully approve, and that does not improve by
adding agents. It is a better ceiling than code review — a misunderstanding
caught in a specification is cheaper than one caught in a merge request — but it
is a ceiling, and it is lower than people expect.

## Opting out

Rule #11 is a convention, not a lock. A service can decline it, and that call
belongs to whoever owns the repository. A rule its owner cannot switch off does
not produce the work it describes — it produces cases written to satisfy a file.

The mechanism already exists and needs nothing built: `CONTRIBUTING.md` overrides
`AGENTS.md` wherever the two disagree, as the generated file's own preamble says,
and `waffle agents` never touches it. So declining is a paragraph there, and it
survives every upgrade.

**The wording is yours.** How you instruct an agent in your own project is not
the framework's business, and nothing here prescribes a form of words. Two
properties of it are worth knowing, because they are what the agent side is told
to look for:

- **An opt-out has to be about this rule.** An agent is instructed not to infer
  one from general remarks about pace — "we move fast here", "tests come after" —
  so a paragraph meant to switch the workflow off has to say that it does. This
  constrains how strictly an agent reads, not how you write.
- **Nothing else falls with it.** The platform boundary is not part of Rule #11:
  `replacements.allow` is enforced by a test in the ordinary suite, and an agent
  may no more grant itself a seam in traditional mode than in this one. Turning
  the workflow off does not hand over the cut.

**Say what replaces it.** "Traditional mode" is not one protocol. If an approved
specification stops being the artifact that says what *correct* means, something
else has to — at a minimum, a test for the behaviour that changed. Leaving that
open reproduces the failure this cycle exists to prevent, by policy instead of by
accident: a production change nothing pins, which the next reader takes for a
decision because the code is the only thing left to read.

**It is not binary.** The rule already exempts two categories on its own — a
code defect at the level of a function takes a table test where the defect is,
and a spike is exempt entirely. Keeping the cycle for features and design
defects while dropping it elsewhere is a smaller step than switching it off, and
it is the shape most teams actually want.

**A repository file and a task are different scopes.** `CONTRIBUTING.md` says how
this service lives, always, and a session cannot choose otherwise. "Quickly, just
this once" is a property of one task and belongs in the task. Putting it in the
repository turns a standing decision into a switch for whatever mood the day is
in.

**What declining gives up.** On kostej — the first service taken through the
cycle end to end, 233 scenarios over 18 operations — writing the cases surfaced
defects in code that was already in production: a grid keeping the wrong version,
freeze boundaries off by a day at both ends, a missing facility requirement, a
currency anchored to the wrong moment, a promo window short by its last day, and
a product kind that could be changed after the fact. None of them needed a test
run to find. They came out of having
to state, per case, what the answer should be — which is the part traditional
mode removes, and it is not the same as the part that runs in CI.

## What this does not cover

A component test with the edges cut verifies one service against its *assumed*
contract with the others. The expensive failures in a fleet are contract drift
and retry or ordering semantics between services — precisely what the cut removes
from the test. This raises confidence inside a service and says nothing about the
seams between them; that needs contract tests or protobuf compatibility checks,
which are a separate mechanism.

## Status

Honest inventory, because a workflow documented ahead of its tooling is a
workflow that gets abandoned at the first step that does not work.

**Available:**

- `waffle tests:harness init` scaffolds the harness, the suite's `TestMain`, the
  boot smoke test and `TestReplacementsStayInsideTheList`, reading the container
  for the database, the transport and the connections that must be dead.
- `replacements.Check()`, scaffolded as a test beside the container, enforces
  `replacements.allow` — what a test may replace is a decision someone made, not
  a habit — and `waffle tests:replacements` prints everything the containers
  expose, with `check` for the same comparison on demand.
- Rule #11 in `AGENTS.md`, which is phrased over "executable specification"
  rather than any one format, so it is followable today with ordinary Go tests.
- **the component harness**, as libraries — see
  [conventions/component-harness](component-harness.md). The resource seam is
  `waffle/resource`; postgres, redis and elasticsearch are `sql/sqltest`,
  `redis/redistest` and `elastic/elastictest`; the in-process servers are
  `waffle/tools/grpc.Serve`,
  chaining interceptors with the same `UnaryChain` the daemon uses, and
  `waffle/tools/http.Serve` over the container's own mux and router; the leaves
  that used to be hand-written per service — a logger, a consumer, a dead
  connection — are `pry.Recorder()`, `scribus.Recorder()` and `grpcClient.Dead()`.
  Four services run on them. A run needs no configuration at all: without a DSN
  the harness starts a postgres container and the next run reuses it.

**Not available yet:**

- **a check that behaviour did not change without its specification.** Decidable
  from a diff: domain service or model files changed, and no `.feature` or
  baseline did. It fails on the case that actually happens — code edited in a
  hurry, no specification touched — and asks for nothing when nothing is wrong.
  Not written yet.

**Deliberately not a gate.** Rule #11 is satisfied by a person reading a
specification and judging it, and no pipeline can check that a person did. A
pipeline can check that a *string* is present, which is worse than checking
nothing: it trains everyone to paste a link, and then the merge request carries a
"specification approved" mark that nobody consciously gave. Approval is an act
inside a session; what has to survive the session is the specification itself, in
the repository, and a link to the ticket where the delta was argued. Only the
first of those is worth a machine check, and the bullet above is its honest form.

What the two services showed, worth knowing before the third: the expensive
parts were not the runner but the decisions around it — binding at the declared
contract rather than one layer below, discovering that replacing a seam does not
prune what feeds it, and noticing that most cases need no clock at all if
fixture dates are set relative to now. Those are written down in Rule #11 so the
third service does not rediscover them.

The extraction itself taught one thing worth repeating: **what deserved to be
shared was not the silent stand-in but the recording one.** A silent leaf is
three lines written on the spot; a spy is the part that gets rewritten badly, or
cannot be written at all — a log spy is impossible outside `pry`, because the
options only unpack inside that package. `Noop()` is a recorder nobody queries.
