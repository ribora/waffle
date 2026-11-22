# `waffle agents`

[Docs](../README.md) · CLI scaffolding

Regenerates `waffle.md` in the current service with the framework's instructions
for AI coding agents (conventions, anti-patterns, command cookbook) plus
auto-detected service context (module path, domains, modules, env vars), and
keeps the line in the service's `AGENTS.md` that brings it in.

```
waffle agents [-q]
```

`AGENTS.md` is the service's own. Where there is none, `waffle agents` writes a
stub holding the line; where one lacks the line, it is added at the end; one that
has it is left alone. An `AGENTS.md` an older waffle wrote whole is replaced by the
stub, since it was waffle's. Also (re)creates symlinks `CLAUDE.md → AGENTS.md` and
`.cursorrules → AGENTS.md` so any major AI tool finds its expected file. Real files
(not symlinks) are never overwritten. A file already current is not rewritten, and
is reported as `= … — up to date`; an `AGENTS.md` holding the line, or a real
`CLAUDE.md` or `.cursorrules`, as `= … — exists, left alone`. `-q` leaves those lines
out, which is how the scaffolded `make generate` runs it on every build — it prints
only when a waffle upgrade or a new domain actually changes something.

## What is in the file

Eleven numbered rules, one per area of the framework: the CLI as the first resort
(#1), `make && make audit` after every edit (#2), the boundary
rule (#12), naming (#3), diogen (#4), whisk (#5), conversions (#6), errors (#7),
observability (#8), the order of steps per task (#9) and the anti-patterns (#10).
Each is the short form of a page under `docs/`, written for someone acting on it
rather than deciding about it. The numbers are identifiers, not a sequence.

**`CONTRIBUTING.md` overrides all of it**, and `waffle agents` never touches that
file. The generated file says this in its preamble, so an agent reads
`CONTRIBUTING.md` first without being told to.

How a change is specified, tested and approved is not the framework's: a service
says so in its own `AGENTS.md`, beside the line that brings `waffle.md` in.

## When to run

| Situation | Action |
|---|---|
| `waffle init:service` runs it automatically | nothing to do — file is fresh |
| **You bumped waffle to a new version** (`go get github.com/ribora/waffle@latest`) | **run `./waffle agents`** — conventions may have changed |
| You added a new domain / waffle module / env var and want waffle.md to reflect it | run `make generate`, which rewrites it; `make:domain` does too |
| You changed business logic only | not required (auto-detected context unchanged) |

`waffle.md` is **fully waffle-managed** — any hand edits are lost on
regeneration. Put service-specific instructions in `AGENTS.md` or
`CONTRIBUTING.md` instead.

## Why it matters

Waffle conventions evolve: new commands, removed APIs, changed metric names, new
lints. Each waffle release ships a fresh `waffle.md` content. A stale one teaches
your AI agents outdated patterns — they will write code that compiles but breaks
`make audit`, or miss new commands that would have saved time.

After every `go get github.com/ribora/waffle@latest`, run `./waffle agents` and
commit the diff. It's part of the upgrade workflow, not a separate task. `./waffle`
is the version in `go.mod`; a `waffle` on `PATH` writes its own version's
conventions.

## What it auto-detects

| Field | Source |
|---|---|
| Module path | `go.mod` |
| Domains | directories under `domain/` |
| Waffle modules | blank imports in `config/di/modules.go` |
| Env vars | every `github.com/ribora/env` reader called with a literal key across `config/`, `domain/`, `cmd/`, plus the variables waffle appends to `.env` for each active module |

Everything else (conventions, cookbooks, anti-patterns) is identical across
services — comes from the waffle template.

## CI integration (optional)

Add a check to your pipeline that `./waffle agents` produces no diff:

```sh
./waffle agents
git diff --exit-code waffle.md AGENTS.md
```

If non-zero, the file in the repo is stale — fail the build and tell the
developer to commit the regenerated one.

## Next

| | |
|---|---|
| The pages the framework rules are the short form of | [DI overview](../di/overview.md), [input parsing](../conventions/input-parsing.md), [DTO conversion](../conventions/dto-conversion.md) |
| The boundary rule behind Rule #12 | [Recommended architectures](../architectures.md#the-boundary-rule) |
| Component tests | [DI overview](../di/overview.md#testing-with-the-container) |
