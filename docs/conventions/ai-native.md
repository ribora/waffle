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
  it. Mocking a connection means answering with encoded protobuf.
- behind a factory that builds its clients from runtime data, the real boundary
  is *below* the leaf and outside the container entirely. A mock of it cannot
  reach the app through `Replacements` at all — it serves a unit test of the
  factory, while the component test still cuts at the factory's own port.

A leaf is also not always worth a mock. `sql.Db` is one, but a component test
hands it a real connection to a test database. Same for a port whose production
implementation already has a null variant selected by data: nothing needs
replacing, so it is classified and left alone.

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
joiner reading code infers intent, while a specification plus a delta list shows
which decisions were deliberate.

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

**Not available yet:**

- **a scenario harness.** Gherkin needs a runner, a database lifecycle
  (migrations onto a test database, data reset between scenarios), a home for
  fixtures and controlled time. None of that ships with waffle, and building it
  per service is the expensive part of step 3. The leverage is to extract it once
  into a shared package rather than pay for it in every service — do that before
  rolling the cycle out widely.
- **a gate.** Rule #11 asks a merge request to carry a link to the approved
  specification. Nothing checks it, and a rule nothing checks is one new joiners
  learn to skip.

Until the harness exists, "executable specification" means a component test in
Go. That is enough for the protocol — approval before code, frozen after — which
is the part that actually does the work. Gherkin is a format; the freeze is the
mechanism.
