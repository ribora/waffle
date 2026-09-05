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

Every field of `Replacements` is a seam — a place a test fills in instead of
letting the container build it. See
[DI overview](../di/overview.md#testing-with-the-container) for the mechanics and
[make:tests:mocks](../cli/overview.md#maketestsmocks) for the tooling.

### Cut at the leaves

`waffle make:tests:mocks --suggest` marks a seam `# leaf` when its default
constructor takes nothing else out of the container. Those are the boundaries of
the process; everything else is a node inside it.

That distinction is the difference between the two kinds of test:

- a **component test** replaces leaves and lets the rest run for real. It is what
  a specification at the service boundary needs.
- replacing an interior seam instead deletes the subtree below it — which is the
  code the specification was meant to exercise. That is a unit test wearing a
  component test's clothes.

A **unit test** does not use `Replacements` at all: it builds the object under
test directly. Interior seams appear in `mocks.generate` for their mocks alone.

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
  unreachable connection, the logger — gets a no-op owned by the harness
  (`pry.Noop()`, `grpcClient.Dead()`). Declaring expectations for the calls a
  container makes while constructing itself turns "this does not matter" into
  work in every case.
- a seam a later assertion **reads back** gets a recorder — `pry.Recorder()`,
  `scribus.Recorder()`. The line between the last two is the specification: a
  no-op is a recorder nobody queries, so silence is right exactly where nothing
  is asserted, and hides the subject everywhere else.

And the negative assertion is written, never implied: `.Times(0)` rather than
leaving the call undeclared and letting the controller fail. The suite is a
document that someone approves, and an assertion that exists only as a missing
line cannot be reviewed.

None of this is derivable, which is why the marker is a fact about the graph and
not a recommendation, and why both `mocks.generate` and `mocks.ignore` are
curated by a person.

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

- `waffle make:tests:component` scaffolds the component test that boots the
  container.
- `waffle make:tests:mocks` generates mocks and, through `--suggest`, `--write`
  and `--check`, keeps the classification of every seam honest.
- Rule #11 in `AGENTS.md`, which is phrased over "executable specification"
  rather than any one format, so it is followable today with ordinary Go tests.
- **the component harness**, as libraries — see
  [conventions/component-harness](component-harness.md). The resource seam is
  `waffle/resource`; postgres and elasticsearch are `sql/sqltest` and
  `elastic/elastictest`; the in-process gRPC server is `waffle/tools/grpc.Serve`,
  chaining interceptors with the same `UnaryChain` the daemon uses; the leaves
  that used to be hand-written per service — a silent logger, a no-op consumer, a
  dead connection — are `pry.Noop()`/`pry.Recorder()`, `scribus.Recorder()` and
  `grpcClient.Dead()`. Two services run on them. A run needs no configuration at
  all: without a DSN the harness starts a postgres container and the next run
  reuses it.

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
