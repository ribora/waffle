# AGENTS.md

Instructions for AI coding agents working in this repository. This file is
**managed by `waffle agents`** — hand edits are lost on regeneration.

**`make generate` rewrites it**, so the service context below — domains, active
modules, env vars — is as current as the container. After a waffle upgrade
(`go get gitlab.com/gorib/waffle@latest`) run it again for the conventions too:
they change between versions, and a stale copy steers you toward patterns that
are gone.

**Read `CONTRIBUTING.md` first, if it exists.** It holds service-specific
instructions — business rules, custom commands, known quirks — and it overrides
this file wherever the two disagree, Rule #11 included. `waffle agents` never
touches it, and it belongs to the humans who own the service: when you learn
something that belongs there — a convention you had to infer, a quirk that cost
you a wrong turn — quote what you would write, and do not write it.

## This service

- Module: `{{ .ModulePath }}`
- Framework: [gorib](https://gitlab.com/gorib) + [waffle](https://gitlab.com/gorib/waffle){{ if .WaffleVersion }} `{{ .WaffleVersion }}`{{ end }}{{ if .Domains }}
- Domains: {{ range $i, $d := .Domains }}{{ if $i }}, {{ end }}`{{ $d }}`{{ end }}{{ end }}{{ if .Modules }}
- Active waffle modules: {{ range $i, $m := .Modules }}{{ if $i }}, {{ end }}`{{ $m }}`{{ end }}{{ end }}{{ if .EnvVars }}
- Env vars: {{ range $i, $e := .EnvVars }}{{ if $i }}, {{ end }}`{{ $e }}`{{ end }}{{ end }}

## The short version

Two kinds of thing are below, and they are read differently. **Rule #11 is a
protocol** — it decides what you are allowed to be doing right now, so it comes
first and it has to be held, not looked up. **The other rules are a reference**
for the framework's conventions; go to the one you need. The numbers are
identifiers, not a reading order.

Held in mind, all of it:

- **Four steps, and you stop after each one.** Cases → *hand over and wait* →
  steps → red-green, with the permission list read before any of them. Being
  asked for one step is not being asked for the next, and an answer you get
  inside a step is not the next step either.
- **Every edit is made in a mode the human switched on.** An edit the mode does
  not allow is a stop: name the mode that allows it and ask (Rule #11, Modes).
- **Three things you do not write.** `waffle.yaml` (the platform owner's,
  platform mode only),
  `CONTRIBUTING.md` (the humans'), and production code before the cases are
  approved. None of them appears in the diff of a specification task.
- **One stop-and-report.** The cut is a given: when it does not fit the case,
  name what you need and hand it back. Working around it is the move that
  destroys the suite's meaning.
- **After every edit,** `make generate && make audit && make`, in that order
  (Rule #2). `make generate` is not optional — diogen fails at generation time,
  not at compile time.

If `CONTRIBUTING.md` says this service works differently, it wins.

## Rule #11 — specification before code

### Whether this rule applies here

It is a convention, and a service may decline it. `CONTRIBUTING.md` overrides
this file, so a paragraph there switching the workflow off is authoritative and
you follow it — the person who owns the repository decides how they want an agent
to work in it, and that is not this file's call.

Three things about reading such a paragraph:

**It has to be about this rule.** Do not infer an opt-out from remarks about
pace — "we move fast here", "tests come after", a short deadline in the task.
Those are not it. If the workflow is off, the file says the workflow is off; if
it does not, this rule is in force and inconvenience is not evidence to the
contrary.

**It is not yours to write.** `CONTRIBUTING.md` is the human's file: propose
content for it, never author or amend it as part of a task. An agent that writes
its own exemption has not found one.

**The platform boundary does not fall with it.** `replacements.allow` is enforced
by a test in the ordinary suite, not by this rule. In traditional mode you may no
more grant yourself a seam than here.

### Who decides what

Two roles, and this rule is written for the first one.

| | owns | in this repository |
|---|---|---|
| the person writing the specification | the cases, the steps, the red-green loop | `tests/`, except `tests/internal/` |
| the person who owns the platform | what may be replaced, and the harness | `waffle.yaml`, `tests/internal/` |

**You work for the first.** The platform is a given: read it, use it, and when it
does not fit the case, say so and stop. Changing a seam to make a case pass
substitutes your judgement for the platform owner's, in the one place where the
whole suite's meaning is fixed.

### Which task this is

| task | what to do |
|---|---|
| a feature, or a **design defect** — an internal contradiction in behaviour that was specified | the full cycle below |
| a **code defect** at the level of a function — code that broke, not behaviour that was contradictory | a table test where the defect is, then the fix. None of the cycle below |
| a **spike** — the requirements are a sketch and the point is to find out what is possible | exempt from the cycle, but its output is a specification, not merged code. The spike's code is thrown away and rewritten through the cycle |

**The cycle has four steps and you stop after each one.** Read `waffle.yaml`
first and check that the list covers what the contract needs; write the cases;
hand them over and wait; only then the steps, and only then red-green. Being
asked for one step is not being asked for the next: "write the scenarios" ends
when the scenarios are written. Neither is an answer you get *inside* a step the
next step. Each stop is a place where a person can change the answer for the
price of a conversation instead of the price of a rewrite.

### Modes

Every edit is made in a mode, and **only the human switches it** — by a command
where the tool has one, otherwise by naming the mode in words: "spec mode",
"platform mode". One mode holds for every service the task names. While a mode is
on, a service the task does not name is closed whole — a mode or a grant for one
service reaches no other; with no mode on, every service is in none. Where the tool enforces these tables, a refused
edit names the mode that would allow it; where it does not, you hold the same
boundary yourself.

| mode | may be edited |
|---|---|
{{ range .Modes }}| {{ .Name }} | {{ .Allows }} |
{{ end }}
The platform's files are `waffle.yaml` and `tests/internal/`. `CONTRIBUTING.md`
is edited in no mode.

An **area** is a named set of paths. In its grant modes the human can open it by
name, until the mode changes. An area with its own modes takes its paths out of
that table and is open in exactly those; one without follows the table, and its
grant only adds. The defaults are waffle's; `waffle.yaml` declares the rest under
`agents.areas`, and the paths may leave the service — a monorepo's shared
contracts are areas. This service has:

| area | paths | open in | granted in |
|---|---|---|---|
{{ range .Areas }}| {{ .Name }} | {{ .PathList }} | {{ .ModeList }} | {{ .GrantList }} |
{{ end }}
`waffle agents:grant <mode>` lists the grants a mode takes. A grant covers exactly
the area it names; needing a second one is a second question. A contract another
service reads is changed in spec mode, because it is part of what gets approved.

**When no mode and no grant fits the work,** say which path you need and in which
mode, and quote the `agents.areas` entry for `waffle.yaml` that would allow it —
its name, paths, and the modes it opens in or is granted in. Adding it is the
platform owner's, in platform mode; the quote is what lets them decide in one
read.

**An edit the mode does not allow is a stop.** Name the mode or the grant that
allows it — `waffle agents:guard <mode> <path>` answers — and ask the human. Do not
decide on your own that the task has left the cycle: leaving it is the human's
call, made by switching the mode. An answer to your question switches nothing,
and neither does an imperative inside a step. `waffle agents:mode <mode>` prints
what a mode allows and which part of this rule it follows; neither command
switches anything.

**Do not test the boundary with an edit.** Ask `waffle agents:guard` first: a
refused edit is the same stop, reached later.

**In Claude Code, the waffle plugin enforces these tables** — `/waffle:<mode>`
switches, `/waffle:allow <area>` grants, and a hook refuses what the mode does
not allow. When it is on, a line saying "waffle plugin active" is in your
context. If it is not, say once, at the start of a Rule #11 task, that the human
can install it, and do not repeat it:

```
/plugin marketplace add https://gitlab.com/gorib/waffle.git
/plugin install waffle@gorib
```

### 0. When the code came first

This service has behaviour nobody wrote down, so the first change to touch it
has nothing to sit inside. Derive a **baseline** from the code first: what it
does today, per declared contract, in the notation the cases will use.

A baseline read out of code inherits the code's defects, so **every case carries
a verdict**, and none is a boundary until it has one:

- **✓** confirmed as intended — a regression boundary.
- **✗** confirmed defect — the case that pins it asserts the *intended*
  behaviour and is therefore red until the fix lands.
- **?** no verdict, so not part of the boundary. Only the human who owns the
  behaviour moves it. Do not guess, and do not mark ✓ because the code is old.

**The verdict belongs to the baseline, not to the specification.** It is the
state of a review, and the baseline is the document that carries one; the
committed cases say what the service does. So no verdict tags on scenarios —
`@open`, `@intended`, `@defect` — and no legend explaining them in a feature
header. A tag like that is review state in the repository: it stops being true
the moment a decision is taken anywhere else, and a stale one reads as current.
A specification is not a task tracker.

What the committed case carries instead is the **reason**: the rule as it stands,
in the present tense, and what it excludes. Not a log — no dated decision
entries, no "fixed on <date>" lines. Three of those and the reader has a history
to work through instead of a rule, and the prose stops being read; a rule that
changes is **rewritten**, not appended to, because who decided it and when is in
the history and in the ticket.

**A red case carries no status.** Its comment is the rule in the present tense,
as if the service already kept it — a new feature and a known defect alike. That
the service does not yet is what the red run says, and it goes in the hand-over,
where the red cases are listed with why each is red; a status line in the file is
wrong the day the case turns green.

**Adjudication is the human's.** Deriving cases is the cheap half, and expect to
be wrong about which are defects — a deliberate business rule reads exactly like
one.

**When a defect is fixed the case flips ✗ → ✓ in the baseline** and its prose is
rewritten to describe the behaviour as it now stands. What used to be wrong is
kept only in the durable form — the implementation the case excludes — and not as
a note about a past state.

**Do not state counts you will not maintain.** "22 cases, 6 defects" in a header
is wrong within two edits, and a stale count reads as current.

### 1. Requirements in, specification out

Produce two artifacts and nothing else yet:

- **an executable specification** — a component test today, a `.feature` file
  once this service has a harness wired. Look in `tests/`; that is the fact,
  whatever any document says. See "Steps and fixtures". The **minimum** set that pins
  down what the requirements left open, not an exhaustive matrix.
- **a delta list** — every place the specification says something the source
  document does not, each pointing at the place it fills in. Include what was
  deliberately left **out** of scope: comparing two documents catches a wrong
  answer and misses a missing question.

The two go to different places. The specification is committed; the delta list
belongs with the ticket, because it is finished at approval. The reasoning for
each case is written **into** the specification.

For a **design defect** the deliverable is the set of cases that cannot all pass
at once. If the contradiction cannot be written as two cases, it is not yet
understood.

**The specification is one artifact, not prose plus code.** Do not produce a
Markdown description and an implementation beside it — that needs verifying
twice, and the second comparison is the one nobody does.

**One notation per service.** Two ways of writing cases in one repository is
load with no payoff. Follow whatever is already in `tests/`; if nothing is, the
choice is the human's, not yours.

**One specification per declared contract, not per service.** The
`interface{ A; B; C }` entry that `waffle tests:replacements`
reports for a shared container field *is* the list of contracts a domain owes —
one constituent per caller, each a place a scenario binds. Cases written
against the gRPC operations alone cover one of them.

### Writing the cases

**A case is verified against the requirements, not the code.** The code tells you
whether a case is red and which names reach the wire; it is the thing under test,
not the answer. A case that disagrees with the code is a red case to report,
never one to align with it. With no requirements in the conversation, ask what
the cases are to be verified against before reading the code for answers.

**A case for a known defect asserts the behaviour you want, so it is red.**
Writing what the code currently does turns a defect into a green regression
boundary. Mechanical check before handing anything over: **the number of red
cases equals the number of known defects.** A mismatch means a defect got frozen
or a case is wrong.

**Name what the case excludes.** For a case of the form "X comes from Y", answer
which implementation would also pass it and should not. If one exists, the case
does not discriminate, and it needs variation — two facilities configured
differently rather than one. Put the answer in the delta list: a reviewer can
judge a named alternative, but not imagine the whole space of them.

**Fold duplicates along one axis only.** Same setup and same trigger, differing
only in the expectation, is one case with a column added. Differing in the
*setup* is not: merging leaves a column empty on most rows, and a sparse column
means two questions were forced into one table.

The same holds when both cases are real. Two cases that differ in one cell and
carry different information are still one table — written as prose they read as
the same case twice, and a reviewer spends the attention on telling them apart
instead of on what they say. Every column of the table has to be substituted
somewhere: a column nobody reads is a placeholder someone forgot to wire, and
the rationale it wanted to carry belongs in the comment above, naming the rows
in order.

**A case must not quietly re-assert a rule that has its own case.** Leaving a
field out of a request and then asserting the default it acquired tests the
default rule for a second time, in a case about something else — so when the
default changes, an unrelated case fails. Fill the cell.

**State the contract facts your cases lean on.** A case whose outcome depends on
something the contract does and no case says — that writing the same key again
edits rather than refuses, that a version is identified by a date — smuggles
that fact in. The reader cannot tell whether it is a rule or an accident, and
the reviewer cannot approve it. Either give it a case of its own or, when
another case already implies it, say it in the rule's comment.

**Assert the meaning, not the representation.** "The contract holds version 1"
pins how a link is stored; "the contract holds terms at 50 percent" pins what
has to survive, and it fails for the right reason when a later version is used
instead. Where both are available, the second is the case and the first is an
implementation detail the specification should not fix.

**Nothing here refers to the baseline.** It is a working artifact and is not
committed, so a reference to it or to its numbering points at a file the reader
does not have. Titles describe behaviour — `Create refuses an empty code`, not
`F2 Create refuses an empty code` — and a comment carries the reason itself
rather than a pointer to where the reason was argued. A verdict tag is such a
pointer: it names a review the reader cannot open. Step 0 says where it lives.

**Language.** Step text and case titles are English — a closed vocabulary, half
of it wire identifiers. The **reasoning** — case comments and the delta list —
is written in the language of the requirements it is verified against, because
a translation inside that comparison is unaudited. Domain identifiers are never
translated: they are the names on the wire.

### 2. Verification, then approval

**Say that the arithmetic was read, not run.** Every number in the cases came
from reading the code, because the steps do not exist yet, and about one in a
hundred will be wrong. Saying so at the gate makes the correction
after the first run an expected event, and tells the reviewer which part of the
document carries the least confidence.

A human reads the source document and the delta list, then corrects or approves.
**No production code before approval.** If a case contains nothing that could
have gone another way, it did not need to be a case.

**Approval is an event, and it has not happened yet.** The task that asked for
the cases is not it: being told to write a specification is being told to write
a specification, and the turn in which you finish it is the turn in which you
stop. Do not start the steps, the harness, the fakes or the production code in
the same breath — hand over what you have, say what is red and why, list what
you need decided, and wait.

**An answer to a question you asked is not a new task either.** Writing the
cases raises questions only a person can settle — which of two behaviours is
wanted, whether a duplicate is rejected or overwrites. The reply comes back as a
bare imperative: "add it", "forbid it", "do B". It reads like a fresh
instruction and it is not one: it settles the content of a row, and that row
still has to be written, handed over and approved. This is how the gate gets
skipped in practice — not by deciding to ignore it, but by losing count of which
step you are on.

### 3. Steps and fixtures

**Bind at the declared contract** — the operation: a handler in `cmd`, a
consumer command, a cron command. Not the management interface below it: one
layer short skips conversion and the error→code map, and makes stand-ins invent
database behaviour (uniqueness, not-found) that belongs to the schema. Fixtures
go in through the container's own repositories, never through the operation under
test.

Then bind to the real container through the harness, which fills in what is a
condition of booting rather than a choice the case makes:

```go
app, err := harness.Boot(ctx, diogen.Replacements{ /* the cut */ })
clients, err := app.Serve()
```

`Boot` wraps `diogen.InitAppWith`; call that directly only on a service with no
harness. The `.feature` files go in `tests/feature/` and the Go beside Go in
`tests/` — only one of the two is a specification, and it is read by someone who
does not read the other.

#### What may be replaced is given, not chosen

`replacements.allow` in `waffle.yaml` is a permission list: what it does not
name may not be replaced, in any form — mock, spy, fake or no-op.
`waffle tests:replacements` prints every seam, granted or not; it is a menu for
the platform owner, not a licence.

**A missing or empty `replacements:` key grants nothing** — not "unrestricted",
and not an oversight you may repair. Check it before writing cases: if the list
and the contract do not meet, **stop there and say so**.

`TestReplacementsStayInsideTheList` fails in the ordinary suite on every field
the list does not cover; `waffle tests:replacements check` prints the same —
run it before handing over and paste what it said. **Its failure names the seams
you need, not an edit to make:** the hand-over is the red output plus the seams
you are asking for. Putting them into `waffle.yaml` turns the check into a
formality that agrees with whatever you did.

`tests/internal/harness/` is read-only too. It supplies what is a condition of
loading rather than a choice — the database, the logger, a dead connection under
a replaced client. Do not change it, and do not move a replacement in there
because a scenario was inconvenient.

**A value one case depends on — an environment variable included — is set by
that case's step, never in the harness.** The harness is suite-wide and the
platform's; putting a case's decision there hides it from the reviewer.

**The form of the replacement is yours**, because it follows from the assertion:

- a **spy** keeps what it was given and is read in a `Then`. A Gherkin step
  asserts after the call, which is the shape a spy answers — so this is the
  default, and the recorders already exist rather than being written per service.
- a **mock** declares before the call, so it fits where the expectation belongs
  to the `Given`. Declare a negative expectation explicitly (`.Times(0)`): an
  assertion that exists only as a missing line cannot be reviewed.
- a **no-op** keeps nothing, so no `Then` can read it.

`docs/conventions/component-harness.md` in waffle has the catalogue — which
recorder exists for which seam — and the reasoning behind a particular grant.
Read it to understand what you were given; it is not a licence to change it.

#### When the cut does not work, stop

Say what you hit and what the specification needs, and hand it back. Do not work
around it. The recognisable cases:

- **a case needs a seam `replacements.allow` does not grant.** Name the
  interface and the assertion it would carry. Adding the grant is not yours.
- **a `Then` cannot see what it must assert** — the seam is a no-op, or a mock
  whose expectation would have to be declared before the scenario knows the
  value.
- **the boot reaches for the environment or the network.** `InitAppWith` is a
  flat eager sequence: a node is skipped only if that node itself was supplied,
  so a replaced typed client still has its connection built beneath it. If the
  harness has not covered that, it is a gap in the harness.
- **a case needs a seam that does not exist** — controlled time is the usual
  one. Where the code compares a fixture's date against `time.Now()`, set the
  fixture relative to now (`now.AddDate(-1, 0, 0)` for "already started") and no
  seam is needed. A case that needs time to *move* does need one, and adding it
  is production code, which this step does not permit.
- **the cut removes what the case was meant to exercise** — a repository
  replaced by a mock makes the test check the mock's SQL, not the schema's.

**Do not build the infrastructure by hand.** `waffle tests:harness` generates
`tests/internal/harness/harness_gen.go` from the container — the database and its
migrations, the transport and a typed client per handler, the interceptor order,
a dead connection for every `ClientConnInterface` seam, a typed field for every
variable the boot reads — and `make generate` regenerates it, in any mode; that
change is not a platform edit. Never edit that file. A run needs no configuration either — with no DSN set, a postgres
container is started and the next run reuses it, so do not ask for one.
`docs/conventions/component-harness.md` in waffle is the reference.

**The boot smoke test comes before any case**, and the first run writes it: it
brings the resources up, boots on the given cut and asserts only that the callers
are non-nil. One run then tells you what the cut is missing instead of an hour
inside a half-written suite.

What the container cannot say for the whole suite — a second resource, a value
every case boots with, `Custom` for a key no code reads through
`gitlab.com/gorib/env` — goes into hooks in `tests/internal/harness/harness.go`,
which is the harness owner's file, not yours.

**A case a step cannot express without a production shape is a stop.** A field or
type the step needs to read is shape, not behaviour: name it, and ask for the
`model` grant — or, where the shape is a contract another service reads, for a
return to spec mode. Filling it is still red-green's.

**The steps are done when every case is red for its own reason.** A case red
because a step reads no column, or a suite that does not compile, is not that —
the stop is not reached, and red-green is not yours to start.

Still no production code at this stage, beyond the shape a grant opens.

### 4. Red-green

The specification and the steps are **frozen** at approval. The loop changes
production code and nothing else.

Never reach green by weakening an assertion, widening a matcher, deleting a case
or relaxing a fixture. If the test cannot go green without touching the
specification or the steps, stop — that is the next section, not a licence.

**A case that was never red has not been shown to fail.** It happens when the
fix and the case land together — a value moved into configuration with the new
number as its default, a condition widened — and the case is green from its
first run. Prove it: put the old value back for one run, check that exactly that
case fails and for its own reason, then restore. Say at the gate that you will,
and say afterwards that you did. A case nobody has seen fail is a decoration,
and the red count check above cannot notice it.

**Every production file in the diff is named by a case that went red.** The
mirror of that count, catching the opposite mistake: a change nobody asked a
case about. Walk the diff file by file at hand-over and say which case failed on
it. A file with no answer is either unnecessary or a behaviour change nothing
pins — and an unpinned change is worse than no change, because it reads as a
decision while the suite would be exactly as green without it. The next person
to open the file will take it for approved behaviour.

**A step can be the thing that is wrong.** A failure that is not on the expected
list is as likely to be a step that does not send what the case says as a defect
in the service — a column added to a table and not read, a value formatted for
the wire the wrong way. Read the failure before believing it: a step defect that
survives makes the case green while it tests nothing.

### Changing an approved specification

Only through the human who approved it, whatever the reason. **Never on your own
judgement.** Two triggers:

- an **internal logical conflict**: two approved cases cannot both hold. State
  the conflict and stop.
- an **external constraint**: the case needs data this service does not own, or
  an ordering the transport does not guarantee. State the constraint and stop.

The human decides what changes — the specification, the scope, or the design.

### What the merge request carries

The **specification**, as a diff, and a **link to the ticket** where the delta
list was argued. Approval itself is not machine-checkable: a pipeline can verify
that a link is present, not that anybody read anything.

**What the diff must not carry.** Files a reviewer can check without reading
anything else:

- **`waffle.yaml`.** Its presence means the task granted itself the permissions
  it was working under.
- **any production file.** Red-green is a separate hand-over with a diff of its
  own; production code arriving together with the cases means the code moved
  before anyone approved what it should do.
- **`CONTRIBUTING.md`.** It is the file that can switch this rule off, which
  makes it the most valuable one to edit and the one nobody should find edited
  here.

Each costs a glance at the file list, and none depends on the author having
followed any of the rules above — which is the point of having them here. Every
other rule in this section is checkable only by someone who read the work.

# Framework conventions

Reference for the framework itself. Read the entry you need; nothing below is a
protocol, and none of it overrides Rule #11.

## Rule #1 — prefer CLI over hand-writing

For any scaffolding task, first check whether a `waffle` command does it. Only
hand-write files when no command matches.

Every command there is, so a gap in this table is a gap in the toolchain and not
something to fill in by hand without saying so. Operands and flags are in
`waffle <command> --help`; a command that leaves work for you prints it:

| Want to do | Command |
|---|---|
| New service | `waffle init:service` |
| New domain (model + service + repository + migrations) | `waffle make:domain <domain>` |
| Add inbound gRPC | `waffle make:domain:grpc <domain>` |
| Add inbound HTTP | `waffle make:domain:http <domain>` |
| Add event consumer | `waffle make:domain:scribus --driver <id> <domain> <event>` |
| Add an entity to a domain | `waffle make:domain:entity <domain> <entity>` |
| Set up migrations for a domain that has none | `waffle make:domain:migrate <domain>` |
| Add one migration to a domain that already has them | `waffle make:domain:migration <domain> <description>` |
| Add CLI subcommand | `waffle make:domain:command <domain> <command>` |
| Make published events survive a crash after the commit | `waffle make:outbox --driver amqp` |
| Add DTO converter (any path) | `waffle make:converter <path>` |
| Put the component harness in place | `waffle tests:harness` |
| See what a test may replace | `waffle tests:replacements` |
| Check what the tests replace | `waffle tests:replacements check` |
| Refresh AGENTS.md | `waffle agents` |

Outbound adapters are the hole: there is **no** command for a client of another
service or of a third-party API. Write it by hand, shaped like an existing one
in `domain/*/adapter/`, and wire it in `config/di/di.go` — see Rule #9.

Each command is idempotent — existing files are NEVER overwritten. You can
re-run any command safely.

Project conventions (API import path, proto directory, DTO mapper) live in
`waffle.yaml` at the project root — generators follow it instead of
hardcoded paths. See `docs/cli/overview.md` in waffle.

## Rule #2 — always run validation after edits

After any change (yours or human's), run **in this order**:

```sh
make generate    # diogen, goverter, whisk, etc. — recompiles all codegen
make audit       # golangci-lint
make             # .tidy, generate, build
```

Never commit between steps. If any of the three fails, fix before proceeding.

**And `make test` before you hand anything over** — once, not after every edit.
It is out of `make` on purpose: the component suite boots the real container
over real infrastructure, which is too slow for a loop that runs after every
change. It is also where `replacements.Check()` and `boundaries.Check()` live,
so the three commands above passing does **not** mean the seams and the
boundaries are clean. Run it, and paste what it said.

`make generate` is mandatory — diogen reads the entire DI graph and may emit
errors that surface only at generation time (missing dependency, circular,
duplicate alias). Skipping it leaves the service in a broken state.

## Project layout

The scaffolds write it, so the shape is what `waffle make:domain` left in
`domain/`. What the tree does not show:

- `domain/<x>/model` depends on nothing, `service/` on `model/` only;
  `adapter/` is outbound and the only door to another domain (Rule #12),
  `cmd/` is inbound
- `api/proto/` is edited; `api/pb/` and every `*_gen.go` are generated
- `config/di/di.go` is yours; `config/diogen/di_gen.go` is diogen's
- `mapper: dto` in `waffle.yaml` scaffolds no `convert/` package — conversions
  are written by hand

## Rule #12 — a domain is reached only through an adapter

`adapter/` is the one door out of a domain, and the neighbour's `model/` is the
one thing behind it. Only another domain's `adapter/**` (for `model/`), the
composition root and `tests/` may import a domain — `pkg/` may not, and domain
logic parked there is a boundary nobody can see. Inside a domain, `service/`
never imports its own `adapter/`: it declares what it needs and DI answers it.

The check reads `di.Wire`/`di.Define`/`di.Alias` as well as imports, so a
cross-domain wiring with no import is caught too. The full rule set is
`docs/architectures.md` in waffle.

The legal shape: the adapter declares the interface it needs (Rule #3), imports
the neighbour's model only for the types in that interface, and translates them
into its own.

```go
// domain/rent/adapter/invoice.go
package adapter

import (
	invoiceModel "{{ .ModulePath }}/domain/invoice/model"
	"{{ .ModulePath }}/domain/rent/model"
)

// Declared here, implemented by invoice/service, wired in config/di/di.go.
type InvoiceManagement interface {
	CreateInvoice(ctx context.Context, customerId int, amount money.Money) (*invoiceModel.Invoice, error)
}

func (s *invoiceService) toDomain(invoice *invoiceModel.Invoice) *model.Invoice { ... }
```

```go
// config/di/di.go — the neighbour's service answers the adapter's interface
di.Define(invoiceService.NewInvoiceManagement,
    di.Alias[invoiceGrpc.InvoiceManagement](),
    di.Alias[rentAdapter.InvoiceManagement](),   // the only cross-domain shape
)
```

Behaviour crosses through the interface; types cross directly and stop at
`toDomain`. Past that line the neighbour's model does not appear.

`boundaries.Check()` runs in `make test`, not in the three commands of Rule #2.
There is no exceptions file: a crossing that would need one is a design
question for the platform engineer, not a line of configuration.

## Rule #3 — naming conventions (strict)

These deviate from Go style guide but are project-wide. Don't "fix" them.

- `Db` not `DB`, `Dsn` not `DSN`, `Id` not `ID` (across all gorib + waffle + services)
- **Constructors return the unexported struct, and the interface is on the
  registration.** `func NewUserHandler(...) *userHandler`, wired as
  `di.Wire[Routes](NewUserHandler)` — that is what every scaffold writes, and
  it is what gives the seam a `Replacements` entry. Exporting the struct only
  adds a second way to reach the same thing. Two exceptions: a constructor
  whose type is the interface (`func NewRouter(...) server.Router`), and a type
  collected by tag and reached no other way (`di.Define(..., di.Tag(...))`),
  where there is nothing to name it by
- Domain interfaces declared by **consumer**, not producer:
  - gRPC handler declares `type FooManagement interface { ... }` — what it needs from service
  - Service implements it without referencing `grpc.FooManagement` — wired via `di.Alias[grpc.FooManagement]()`
- File naming: snake_case for files (`back_in_stock.go`), CamelCase for types

## Rule #4 — dependency injection (diogen)

DI is **compile-time**: `config/di/di.go` is read by diogen and turned into
`di_gen.go`; there is no runtime container. The primitives and every generation
error are in the diogen README — a mistake in wiring stops `make generate` with
a message the README's Troubleshooting section explains.

What generation does not stop, because it compiles and is wrong:

- ❌ Calling constructors by hand outside `InitDi()` or its delegates
- ❌ `func init()` for DI registration — registrations go in `InitDi()`
- ❌ Registering a waffle module in `di.go` — it is a **blank import** in
  `config/di/modules.go`, then `make generate`
- ❌ `if env.Value("APP_ENV") == "test"` in `InitDi()` — test knowledge in
  production wiring. A test substitutes through `Replacements` (Rule #11 step 3)
- ❌ `di.Override()` to inject a mock — it is a build-time directive, so the mock
  would have to be importable from production code

**Supplying a `Replacements` field does not prune what fed that constructor.**
Whatever is eagerly built below it still gets built, environment and network
included. `di_gen.go` says which case a constructor is in, right above the call:
`// skipped once <Field> is supplied` or `// always builds regardless of
replacements: ...`.

## Rule #5 — input parsing (whisk)

Anything the service accepts — HTTP body, query, header, AMQP delivery, a file —
is parsed by code whisk generates from struct tags (`//go:generate go tool whisk
$GOFILE`). Tag syntax, custom validators and the hooks are in the whisk README.

**Before writing tags, run `go tool whisk tags`.** It prints every tag valid
here, built-in and this project's own. An invented tag fails generation; the
reverse — a check that already exists, written again by hand in a handler — is
caught by nothing.

- ❌ Reading `src.Body()` and unmarshalling by hand — `scan.Into[T](src)`, or
  `server.Bind` to receive the parsed `*T`
- ❌ `server.Handle` for a typed DTO — only a `Bind` route reaches the OpenAPI
  document

A rule tags cannot express goes in a hand-written `validate()` or `parse(src)`
method on the DTO. That is the sanctioned hatch, not a failure of the generator.

## Rule #6 — DTO conversion

Default (`mapper: goverter`): each transport gets a generated `convert/`
package. It is the default because goverter **refuses to generate** on a field
it cannot map, so a field added to a model cannot silently stop reaching the
wire — a hand-written mapper compiles happily and drops it.

`waffle make:converter <path>` writes the `convert.go` with its directives
already on it — one method per type pair is all you add. Overrides and the
whitelist are in `docs/conventions/dto-conversion.md` in waffle.

- ❌ Listing `extend` lines one per helper — a single wildcard extend points at
  the project-wide whitelist in `pkg/goverter/convert.go`, and a new helper is
  one line there
- ❌ A hand-written `convert.go` when the struct has ≥5 fields or nested types

## Rule #7 — errors

- `failure.Wrap(format, ...args)` / `failure.New(format, ...args)` — for errors that **cross service boundaries** (gRPC, AMQP, HTTP response bodies). Stable JSON wire format
- `errors.New("...")` / `fmt.Errorf("%w: ...", parent, ...)` — internal errors that don't leave the process
- **Domain-level sentinel errors: `failure.New`, never `errors.New`.** This is
  what `waffle make:domain` writes into `model/errors.go`, and the reason is the
  wire: `failure.New` registers a slug, and the slug is the `type` the HTTP
  error renderer puts in the body.

```go
// domain/user/model/errors.go
var ErrUserNotFound = failure.New("user is not found")
// → {"code":500,"message":"user is not found","payload":{},"type":"user_is_not_found"}
```

A domain sentinel declared with `errors.New` reaches the client as
`"type":"internal_error"` — one label for every failure the service can have,
so no caller can tell them apart. A sentinel is domain-level precisely because
it does leave the process.

## Rule #8 — observability

`hilton.Provider` is in ctx automatically — propagated by transport middleware
(`http.server/middleware`, `grpc.server/interceptor`, `amqp/interceptor`).

Custom span inside a handler:

```go
ctx, span := hilton.ProviderFromContext(ctx).
    Tracer("my-service").
    Start(ctx, "doWork", hilton.WithSpanKind(hilton.SpanKindInternal))
defer span.End()
```

Metrics: histogram only, no separate counters. `db.client.operation.duration_count`
already exists as `_count` aggregate — don't add `db.client.operations` counter.

## Rule #9 — order of steps by task

Only tasks a command does not finish on its own — where the order, or what the
scaffold leaves for you, is the part that is not obvious. A task that is one
command is in Rule #1's table and not repeated here. Each of these ends with
Rule #2 — `make generate && make audit && make`.

| task | order |
|---|---|
| gRPC method on an existing domain | rpc in `api/proto/<x>/service.proto` → regenerate stubs (Rule #10) → method on `cmd/grpc/service.go` → the method on the **handler's** Management interface → implement it in `service/` → conversion in `cmd/grpc/convert/`, or by hand in the `grpc` package under `mapper: dto` |
| database table | `waffle make:domain:migration <domain> <description>` scaffolds the migration only → write the `.sql` → `model.X` → `stored<X>` in `adapter/postgres/stored_<x>.go` |
| outbound client — the one shape with no command | `adapter/<name>.go` by hand, shaped like an existing one → `waffle make:converter domain/<x>/adapter/<name>/dto` → `di.Wire[<x>service.<Name>Client](<x>adapter.New<Name>Service, di.Args(...))` in `config/di/di.go`, with the interface declared by the consumer (Rule #3) |

## Rule #10 — what NOT to do

- ❌ `protoc` directly — proto generation is driven by the project's proto step (in-module: `go generate ./api/...`; monorepo with `proto_dir` outside the module: a central step such as `make -C <api-dir>`)
- ❌ Manual `go mod tidy` after every edit — `make .tidy` does it consistently
- ❌ A `replace` in `go.mod` whose target is outside this repository — a local checkout of gorib or of anything else builds only on the machine it was written on. Release the module and bump the version instead; a `replace` onto another module of the same repository is fine
- ❌ Touch `di_gen.go`, `*_goverter_gen.go`, `*_whisk_gen.go`, `stored_*_gen.go`, `*_mock*.go` — they're generated
- ❌ Wrap stdlib types in custom abstractions inside `domain/<x>/model/` — model stays pure Go types
- ❌ Reach another domain other than through `adapter/`, or import `domain/<x>/cmd/**` from outside `cmd/` (Rule #12)
- ❌ Pass `context.Background()` deeper than the entrypoint — propagate ctx from caller
- ❌ `panic(err)` anywhere past the entrypoint — return the error

## Reference

Read the copy this service depends on, not the one on `main` — conventions
change between versions.

{{ if .WaffleVersion }}- Waffle docs: `$(go env GOMODCACHE)/gitlab.com/gorib/waffle@{{ .WaffleVersion }}/docs/`
{{ else }}- Waffle docs: `gitlab.com/gorib/waffle/-/blob/main/docs/`
{{ end }}
- Any other gorib module — diogen (`gitlab.com/gorib/di`), whisk, the core libs:
  `README.md` in the directory `go list -m -f '{{"{{"}}.Dir{{"}}"}}' gitlab.com/gorib/<module>` prints

