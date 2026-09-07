package main

import (
	"strings"
	"testing"
)

// An empty directory on purpose: a command that lets the extra word through
// reaches for go.mod and fails with something else, which must not read as a pass.
func TestEveryCommandRefusesAnExtraOperand(t *testing.T) {
	t.Chdir(t.TempDir())
	cmds := newCommands()

	for code, cmd := range cmds.All() {
		err := cmd.Run(t.Context(), "a", "b", "c", "d")
		if err == nil {
			t.Errorf("waffle %s accepted four operands", code)
			continue
		}
		// Not `"d"`: each command names its own first extra operand.
		if !strings.Contains(err.Error(), "unexpected operand") {
			t.Errorf("waffle %s: %v\nwant a refusal naming the operand with no slot", code, err)
		}
	}
}

func TestEveryCommandRefusesAnUnknownFlag(t *testing.T) {
	t.Chdir(t.TempDir())
	cmds := newCommands()

	for code, cmd := range cmds.All() {
		err := cmd.Run(t.Context(), "--not-a-flag")
		if err == nil {
			t.Errorf("waffle %s accepted an unknown flag", code)
			continue
		}
		if !strings.Contains(err.Error(), `unknown flag "--not-a-flag"`) {
			t.Errorf("waffle %s: %v\nwant a refusal naming the flag", code, err)
		}
	}
}
