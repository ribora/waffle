---
description: Switch a waffle service to Rule #11 steps mode — approves the cases, freezes them, binds them
disable-model-invocation: true
argument-hint: "[./path/to/service… or ./...]"
---

The waffle hook has answered this command in this turn's context: which services
were switched and what steps mode allows. If it says a service was not switched,
tell the human why and stop.

Arguments: $ARGUMENTS

Switching to steps mode is the approval of the specification handed over in
spec mode, and freezes it. Work as AGENTS.md Rule #11 step 3 says, and stop when
the cases run red.
