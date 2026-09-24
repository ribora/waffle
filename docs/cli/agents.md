# `waffle agents`

[Docs](../README.md) · CLI scaffolding

Regenerates `AGENTS.md` in the current service with the project-wide AI-agent
instructions (framework conventions, anti-patterns, command cookbook) plus
auto-detected service context (module path, domains, modules, env vars).

```
waffle agents [-q]
```

Also (re)creates symlinks `CLAUDE.md → AGENTS.md` and `.cursorrules → AGENTS.md`
so any major AI tool finds its expected file. Real files (not symlinks) are
never overwritten. A file already current is not rewritten, and is reported as
`= … — up to date`; `-q` leaves those lines out, which is how the scaffolded
`make generate` runs it on every build — it prints only when a waffle upgrade or
a new domain actually changes something.

## What is in the file

Twelve numbered rules, and they are not one kind of thing. The file says so in
its own opening: **Rule #11 is a protocol** and the other eleven are reference.

**Rule #11 is where the specification-first workflow actually lives.** The two
roles and what each owns, the three steps with a stop after every one, the
verdicts a baseline carries when the code came first, why an answer to your own
question is not a new task, what may not appear in the diff of a specification
task. It is the longest thing in the file by a wide margin, and it is the
document to read when you want the protocol itself rather than the argument for
it — [conventions/ai-native](../conventions/ai-native.md) is the argument, this
is the instruction.

The rest is the framework, one rule per area: the CLI as the first resort
(#1), `make generate && make audit && make` after every edit (#2), the boundary
rule (#12), naming (#3), diogen (#4), whisk (#5), conversions (#6), errors (#7),
observability (#8), the order of steps per task (#9) and the anti-patterns (#10).
Each is the short form of a page under `docs/`, written for someone acting on it
rather than deciding about it.

**`CONTRIBUTING.md` overrides all of it**, and `waffle agents` never touches that
file. That is the mechanism by which a service declines Rule #11, or anything
else here — see [ai-native / Opting out](../conventions/ai-native.md#opting-out).
The generated file says this in its preamble, so an agent reads
`CONTRIBUTING.md` first without being told to.

Reading it costs nothing: it is `AGENTS.md` in the root of your own service.

## When to run

| Situation | Action |
|---|---|
| `waffle init:service` runs it automatically | nothing to do — file is fresh |
| **You bumped waffle to a new version** (`go get gitlab.com/gorib/waffle@latest`) | **run `waffle agents`** — conventions may have changed |
| You added a new domain / waffle module / env var and want AGENTS.md to reflect it | run `waffle agents` |
| You changed business logic only | not required (auto-detected context unchanged) |

The file is **fully waffle-managed** — any hand edits are lost on regeneration.
Put service-specific instructions in `CONTRIBUTING.md` instead.

## Why it matters

Waffle conventions evolve: new commands, removed APIs (`di.Include` → blank
imports), changed metric names, new lints. Each waffle release ships a fresh
`AGENTS.md` content. A stale AGENTS.md teaches your AI agents outdated
patterns — they will write code that compiles but breaks `make audit`, or
miss new commands that would have saved time.

After every `go get gitlab.com/gorib/waffle@latest`, run `waffle agents` and
commit the diff. It's part of the upgrade workflow, not a separate task.

## What it auto-detects

| Field | Source |
|---|---|
| Module path | `go.mod` |
| Domains | directories under `domain/` |
| Waffle modules | blank imports in `config/di/modules.go` |
| Env vars | `env.Value` / `env.NeedValue` calls across `config/`, `domain/`, `cmd/` |

Everything else (conventions, cookbooks, anti-patterns) is identical across
services — comes from the waffle template.

## Modes, areas and grants

Rule #11 works in modes — `spec`, `redgreen`, `platform`, and `none`
when no mode is on — and each allows editing a fixed set of paths. Only the human
switches the mode, and one mode holds for every service a task names; while it
is on, a service the task does not name is closed whole. The mode
table covers the layout waffle scaffolds and lives in waffle; it is rendered into
`AGENTS.md`, so the file an agent reads and the check a tool runs are the same
table at the same version.

**Areas** are for everything else: a named set of paths, with two fields that
both list modes and mean different things.

- **`grant`** — the modes in which the human can open the area by name,
  `/waffle:allow <name>` in the plugin. Until then it is closed there. A grant
  lasts until the mode changes, and opens that area and nothing more.
- **`modes`** — the modes the area is open in by itself. Leave it out and the
  mode table decides as it would for any path, so `grant` only adds to it. Set
  it, and the area takes its paths out of the table and is open in exactly those
  modes; `modes: []` closes it in every mode.

A refused edit names every way to it: the modes that open the path, and "the X
grant in Y mode" for each grant.

The two exist side by side because they answer different questions. `modes` is
whose work this normally is; `grant` is the exception a human allows knowingly.
`go.mod` is the platform's, so `gomod` is open in platform mode; a feature that
needs a dependency gets it in red-green when the human grants it, not merely
because red-green is on.

waffle ships three areas, and `waffle.yaml` replaces any of them by naming it, or
declares more:

| default | paths | open in | granted in | for |
|---|---|---|---|---|
| `gomod` | `go.mod`, `go.sum` | platform | redgreen | a dependency the feature needs |
| `steps` | `tests/*_test.go` | as the table says | platform | steps that follow a harness change — the case verdicts before and after must match |
| `proto` | `proto_dir` | spec, platform | — | the contract; in a monorepo `proto_dir` already points at the shared one |

```yaml
agents:
  areas:
    events:                     # a monorepo's shared contract
      paths: [../../shared/api/events/]
      modes: [spec, platform]
    shared:
      paths: [../../shared/pkg/]
      modes: [platform]
```

Paths are relative to the service and may leave it; a trailing slash means
everything below, and `*` matches one directory. Outside the service the mode
table does not apply, so an area there needs `modes` to be open anywhere. A
contract another service reads belongs in spec mode, because changing it is part
of what gets approved. A path outside the service that no area names is not this
service's to decide.

```
waffle agents:guard <mode> <path> [--grant <name>]
waffle agents:mode <mode>
waffle agents:grant <mode> [name]
```

`agents:guard` exits 0 when the mode — with the grants passed — allows editing
the path, and 1 otherwise, naming the mode or grant that would. A path outside
the service that no area names prints `unclaimed` and exits 0. It is the check a
tool runs before an edit; it keeps no state and switches nothing, so which mode
is on and what was granted is the tool's to track.

`agents:mode` prints what a mode allows and which part of Rule #11 it follows —
the text a tool shows the agent when the human switches. `agents:grant` lists
the areas a mode takes as grants, or checks a named one and says what it opens.

**When nothing fits.** If the work needs a path no mode and no grant opens the
way it needs, the agent is told to stop and quote the `agents.areas` entry that
would allow it. Adding it is the platform owner's, in platform mode — the file
is theirs, and the quote lets them decide in one read.

Without a tool that calls these commands, nothing changes: the agent reads the
same tables in `AGENTS.md` and holds the boundary itself.

## CI integration (optional)

Add a check to your pipeline that `waffle agents` produces no diff:

```sh
waffle agents
git diff --exit-code AGENTS.md
```

If non-zero, the AGENTS.md in the repo is stale — fail the build and tell the
developer to commit the regenerated file.

## Next

| | |
|---|---|
| Why the specification-first cycle exists, and what it costs | [conventions/ai-native](../conventions/ai-native.md) |
| The component test Rule #11 asks for | [conventions/component-harness](../conventions/component-harness.md) |
| What a test may replace, which Rule #11 treats as read-only | [cli/overview](overview.md#testsreplacements) |
| The pages the framework rules are the short form of | [DI overview](../di/overview.md), [input parsing](../conventions/input-parsing.md), [DTO conversion](../conventions/dto-conversion.md) |
| The boundary rule behind Rule #12 | [Recommended architectures](../architectures.md#the-boundary-rule) |
