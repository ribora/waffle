# `waffle agents`

[Docs](../README.md) · CLI scaffolding

Regenerates `AGENTS.md` in the current service with the project-wide AI-agent
instructions (framework conventions, anti-patterns, command cookbook) plus
auto-detected service context (module path, domains, modules, env vars).

```
waffle agents
```

Also (re)creates symlinks `CLAUDE.md → AGENTS.md` and `.cursorrules → AGENTS.md`
so any major AI tool finds its expected file. Real files (not symlinks) are
never overwritten.

## What is in the file

Twelve numbered rules, and they are not one kind of thing. The file says so in
its own opening: **Rule #11 is a protocol** and the other eleven are reference.

**Rule #11 is where the specification-first workflow actually lives.** The two
roles and what each owns, the four steps with a stop after every one, the
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
