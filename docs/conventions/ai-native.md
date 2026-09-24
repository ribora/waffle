# AI-native development

[Docs](../README.md) · Conventions

How a change gets made when a coding agent writes most of the code.

The premise is a shift in what is scarce. Writing code is no longer the
bottleneck; deciding what *correct* means is. So the human artifact of a change
stops being reviewed code and becomes an approved specification, and the agent's
job is to satisfy it without being able to move the goalposts.

**The protocol itself is Rule #11 in `AGENTS.md`** — the generated file in the
root of your own service, written for the agent that works to it: the roles,
the three steps and the stop after each, what may not appear in the diff. This
page is the other half — why the cycle is shaped that way, what the framework
gives you, and, honestly, what is not built yet. Read this to decide; read Rule
#11 to follow. [cli/agents](../cli/agents.md#what-is-in-the-file) describes the
file and how to keep it current.

## The cycle

```
requirements ──▶ specification + delta list ──▶ approval ──▶ red-green
                          ▲                                       │
                          └─────── conflict or constraint ────────┘
```

1. **Requirements in, specification out.** The agent produces an executable
   specification and a *delta list*: every place the specification says something
   the source document does not, plus what was deliberately left out of scope.
2. **Verification, then approval.** A human reads the source document and the
   delta, and corrects or approves. No production code before approval.
3. **Red-green.** The specification is frozen. Steps bound to the real container
   run every case red for its own reason before any behaviour is written; then
   production code until they pass.

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

### The review surface, and the two failures that shaped it

What makes Rule #11 enforceable is not its wording. It is a **review surface that
does not depend on the author's compliance**, and it costs two glances:

- `waffle.yaml`, production files and `CONTRIBUTING.md` do not appear in the diff
  of a specification task — one look at the file list;
- the number of red cases equals the number of known defects, and every
  production file in the diff is named by a case that went red — two numbers and
  a list.

Neither was designed up front. Each came out of an incident where an agent broke
the rule with the rule in front of it, and the second incident refutes the fix
everyone reaches for first.

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

**Prose was tried first, and one line of it made things worse.** After the first
failure the fix was a paragraph of argument, and in it "run the check before you
hand anything over" turns a red check into a list of the exact lines that would
make it green. The file-list surface is what actually holds — it does not depend
on the agent reading the paragraph the right way round. The second failure is
what extended it to production files and to `CONTRIBUTING.md`, and what added
the red-count check's mirror.

The line itself stayed, because running the check *is* right; what it needed was
the sentence after it. Rule #11 now says "its failure names the seams you need,
it is not a list of edits to make", and names the forbidden one of the two ways
to turn the output green. Worth noting as a pattern: the fix for prose that
points the wrong way is usually the next sentence, not deleting it — but neither
is what makes the rule hold.

The generalisation, now with two data points: **prose describing a state does not
stop an agent with momentum. A prohibition needs either a mechanism or a surface
a third party can check.** Both incidents were argued against, at length, in the
file the agent had read.

### Modes: the third failure, and the mechanism

**Third: leaving the cycle by classifying the task.** In the specification step,
an agent was answered "fix `waffle.yaml` for the new layout". It judged —
plausibly — that a generator setting is not behaviour and so not a specification
task, declared the request outside the step, and edited the file. It broke
nothing it was told in so many words: it had been told that an answer is the
content of a row, and it decided this answer was not one. Prose lost a third
time, and this time to an argument about which rule applies.

So the step boundary became a mechanism. Every edit is made in a **mode** —
`spec`, `redgreen`, `platform`, or none — and each allows a fixed set of
paths. Only the human switches the mode. The table lives in waffle: `AGENTS.md`
renders it, and `waffle agents:guard <mode> <path>` answers from the same copy,
naming the mode that would allow a refused path. The agent's move stops being
"decide whether this is still the task" and becomes "ask for the mode" — a
question the human answers in one word.

The first use bore this out. Told in spec mode to add two kinds to the fiscal
policy, the agent said the change was production code and not this step's — and
tried the edit anyway. The hook refused it; the agent asked for redgreen by its
exact command, and when then told to "ignore it and be quick", answered that the
refusal was not its to skip. Prose it had just quoted did not stop the edit; the
mechanism did.

**Then the switching was measured, and one boundary went.** Twenty-five sessions
from before the mechanism were set against four with it. The violations a mode
would refuse had cost about 3% of the human's messages; in the four, about a
third of the messages were mode commands, and one refusal in the repository
stopped a real mistake. The class worth a mechanism is production code before the
specification is approved — the costliest one a mode catches. The boundary
between steps and red-green caught nothing that had cost a message, and produced
failures of its own: a step that needed a shape it could not add, and an agent
steering towards whichever mode opened its next edit. So they are one mode now.
The cases stay frozen, the red run still comes before any behaviour, and a step
changed after it is named at hand-over.

Two decisions in it are deliberate:

- **The rules stay in `AGENTS.md`.** The mode table is part of Rule #11, so an
  agent in any tool — or in a cloud session, where plugins do not load — works to
  the same protocol and holds the boundary itself. A tool adds enforcement, not
  content.
- **The table is versioned with the service.** `agents:guard` is the service's
  own waffle, so services on different waffle versions each get their own rules,
  and whatever calls it holds none.

### The Claude Code plugin

For Claude Code the mechanism ships as a plugin in this repository. It adds
`/waffle:spec`, `/waffle:redgreen`, `/waffle:platform` and `/waffle:none`, which
only the human can invoke; keeps one mode per session for every service the task
names; and refuses an edit the mode does not allow before it happens, with the
mode to ask for. `/waffle:allow <area>` widens the current mode by one named area
until the mode changes — `gomod` in redgreen, for a dependency — so a feature
that needs a library is one word rather than a round trip through platform mode.
The areas themselves, a monorepo's shared contracts
among them, are declared in `waffle.yaml` —
[cli/agents](../cli/agents.md#modes-areas-and-grants). A service whose waffle has no `agents:guard` yet stays
editable, and the plugin says once that edits there are not checked.

For yourself:

```
/plugin marketplace add https://gitlab.com/gorib/waffle.git
/plugin install waffle@gorib
```

For everyone on a project, in its `.claude/settings.json`; a teammate gets the
plugin when they trust the folder:

```json
{
  "extraKnownMarketplaces": {
    "gorib": {"source": {"source": "url", "url": "https://gitlab.com/gorib/waffle.git"}}
  },
  "enabledPlugins": {"waffle@gorib": true}
}
```

In a monorepo this goes in the root's `.claude/settings.json` — Claude Code does
not read one in a subdirectory. The plugin finds the service from the path of the
file being edited, and a command names one by its path, relative to where the
session runs — `/waffle:spec ./path/to/service`, several at once for a change
that crosses services, or `./...` for every service below a directory, as
`go test ./...` names packages. It is always a path: waffle does not know how a
repository lays its services out. The path is typed once per task: a command
that names none acts on the services the session named last, or on the one the
session runs in. While a mode is on, a service the task does not name is closed
whole, so platform mode for one service cannot touch another; a fix across the
fleet is `/waffle:platform ./...`.

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

- **One specification per declared contract, not per service.** A domain's
  management can serve four callers — gRPC, a consumer and two cron commands —
  with no overlap at all, so a specification written against the gRPC operations
  alone would cover one contract in four. The
  `interface{ A; B; C }` entry that `waffle tests:replacements` prints
  for a shared container field *is* that list of contracts.
- **Adjudication of defects is the human's.** Deriving candidate defects from
  code is the cheap half and it is wrong often: on the services this was first
  built on, several proposed defects turned out to be deliberate business rules
  and one could not be reproduced at all.
- **The red-count check earns its place.** "The number of red cases equals the
  number of known defects" has caught a real mistake more than once — a defect
  frozen as a green boundary, and a case that was simply wrong.
- **A case can fail to discriminate and still look complete.** Three cases
  pinning "the adapter is asked with the facility's own configuration" all used
  one facility with one provider, so an implementation holding a single
  configuration for the whole process passed all three. The second facility is
  what turned them into cases.
- **Hand-computed numbers fail at about one in a hundred.** One row of a hundred
  and four had the wrong date — month-end clamping done in the head — and the
  code was right. Which is why the gate is told that the arithmetic was read
  rather than run.
- **An unexpected failure is as likely to be the step as the service.** One of
  sixteen: a column added to a fixture table that the step never read.

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

Where to cut is a question with its own page —
[conventions/component-harness](component-harness.md#choosing-where-to-cut) has
the `# leaf` marker, the two directions in which it misses the real process
boundary, and which stand-in each interface gets. What matters *here* is why the
specification depends on the answer.

A specification is stated at the boundary of the service: given this state, when
this call arrives, then this happens. It is satisfied only if the code between
the call and the outcome actually runs. So:

- a **component test** replaces leaves and lets the rest run for real. That is
  the only arrangement in which the specification is checking what it claims to.
- replacing an **interior** seam deletes the subtree below it — which is the code
  the specification was written about. The suite stays green, the specification
  stops being evidence, and nothing about the diff says so. It is a unit test
  wearing a component test's clothes.
- a **unit test** builds its subject directly and never touches `Replacements`,
  so the permission list has nothing to say about it.

Two consequences are the specification's own, not the harness's.

**The negative assertion is written, never implied.** `.Times(0)`, rather than
leaving the call undeclared and letting the controller fail on it. The suite is
a document somebody approves, and an assertion that exists only as a missing
line cannot be approved — the reviewer would have to notice an absence.

**What is deliberately not replaced is stated too.** A port whose production
implementation already has a null variant selected by data needs no stand-in; if
that is not written down, the next reader cannot tell a decision from an
oversight, which is the same failure in a different place.

None of it is derivable, which is why `replacements.allow` is curated by a
person and the marker is a fact about the graph rather than a recommendation.

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

**What declining gives up.** On the first service taken through the cycle end to
end, writing the cases surfaced defects in code that was already in production: a
grid keeping the wrong version, freeze boundaries off by a day at both ends, a
missing facility requirement, a currency anchored to the wrong moment, a promo
window short by its last day, and a product kind that could be changed after the
fact. None of them needed a test run to find. They came out of having to state,
per case, what the answer should be — which is the part traditional mode removes,
and it is not the same as the part that runs in CI.

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

- `waffle tests:harness` generates the harness from the container — the
  database, the transport and its clients, the connections that must be dead,
  the environment — and regenerates it on every `make generate`. Its first run
  scaffolds the suite's `TestMain`, the boot smoke test, the migrations check
  and `TestReplacementsStayInsideTheList`.
- `replacements.Check()`, scaffolded as a test beside the container, enforces
  `replacements.allow` — what a test may replace is a decision someone made, not
  a habit — and `waffle tests:replacements` prints everything the containers
  expose, with `check` for the same comparison on demand.
- Rule #11 in `AGENTS.md`, which is phrased over "executable specification"
  rather than any one format, so it is followable today with ordinary Go tests.
- Rule #11's modes, `waffle agents:guard` and `waffle agents:mode`, and the
  Claude Code plugin that enforces them — see
  [the plugin](#the-claude-code-plugin).
- **the component harness**, as libraries — see
  [conventions/component-harness](component-harness.md). The resource seam is
  `waffle/resource`; postgres, redis and elasticsearch are `sql/sqltest`,
  `redis/redistest` and `elastic/elastictest`; the in-process servers are
  `waffle/tools/grpc.Serve`,
  chaining interceptors with the same `UnaryChain` the daemon uses, and
  `waffle/tools/http.Serve` over the container's own mux and routers; the leaves
  that used to be hand-written per service — a logger, a consumer, a dead
  connection — are `pry.Recorder()`, `scribus.Recorder()` and `grpcClient.Dead()`.
  Services run on them, which is what puts this under "available" rather than
  under the list below. A run needs no configuration at all: without a DSN the
  harness starts a postgres container and the next run reuses it.

**Not available yet:**

- **a check that behaviour did not change without its specification.** Decidable
  from a diff: domain service or model files changed, and no `.feature` or
  baseline did. It fails on the case that actually happens — code edited in a
  hurry, no specification touched — and asks for nothing when nothing is wrong.
  Not written yet.
- **edits made through the shell.** The plugin checks the editing tools; a file
  rewritten by a shell command is not checked.
- **the task type, declared by the human.** A code defect and a spike take no
  cycle, and the agent still decides which a task is — so a feature relabelled
  as a code defect skips the cycle with no mode switched. Modes for a fix and a
  spike would make that the human's call; they wait on the plugin's first use.
- **enforcement in other tools.** Kiro, Antigravity and the rest read
  `AGENTS.md` and hold the modes themselves; `agents:guard` takes plain
  arguments, so an adapter for any of them calls the same command.

**Deliberately not a gate.** Rule #11 is satisfied by a person reading a
specification and judging it, and no pipeline can check that a person did. A
pipeline can check that a *string* is present, which is worse than checking
nothing: it trains everyone to paste a link, and then the merge request carries a
"specification approved" mark that nobody consciously gave. Approval is an act
inside a session; what has to survive the session is the specification itself, in
the repository, and a link to the ticket where the delta was argued. Only the
first of those is worth a machine check, and the bullet above is its honest form.

What the first services through this showed, worth knowing before yours: the
expensive parts were not the runner but the decisions around it — binding at the
declared contract rather than one layer below, discovering that replacing a seam
does not prune what feeds it, and noticing that most cases need no clock at all
if fixture dates are set relative to now. Those are written down in Rule #11 so
they are not rediscovered.

The extraction itself taught one thing worth repeating: **what deserved to be
shared was not the silent stand-in but the recording one.** A silent leaf is
three lines written on the spot; a spy is the part that gets rewritten badly —
and every service that needed one wrote its own until these existed. `Noop()` is
a recorder nobody queries.
