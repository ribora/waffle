# `waffle agents`

Regenerates `AGENTS.md` in the current service with the project-wide AI-agent
instructions (framework conventions, anti-patterns, command cookbook) plus
auto-detected service context (module path, domains, modules, env vars).

```
waffle agents
```

Also (re)creates symlinks `CLAUDE.md → AGENTS.md` and `.cursorrules → AGENTS.md`
so any major AI tool finds its expected file. Real files (not symlinks) are
never overwritten.

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

## Related

- [Conventions / DTO conversion](../conventions/dto-conversion.md) — referenced from AGENTS.md
- [DI overview](../di/overview.md) — referenced from AGENTS.md
