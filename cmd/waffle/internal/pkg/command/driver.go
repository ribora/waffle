package command

import (
	"fmt"
	"os"
	"slices"
	"strings"
)

const ModulesFile = "config/di/modules.go"

// Runs before Operands counts them, or the flag and its value are operands.
func TakeDriver(args []string) (string, []string, error) {
	for i, arg := range args {
		if id, found := strings.CutPrefix(arg, "--driver="); found {
			return id, slices.Delete(args, i, i+1), nil
		}
		if arg != "--driver" {
			continue
		}
		if i+1 == len(args) {
			return "", nil, fmt.Errorf("--driver takes the broker module's name, e.g. --driver amqp")
		}
		// Read before the delete: it shifts the tail left, and which of the two
		// runs first in one return statement is not ours to decide.
		id := args[i+1]
		return id, slices.Delete(args, i, i+2), nil
	}
	return "", args, nil
}

// Before the first file is written: UncommentModule uncomments without adding, so
// a driver whose line is absent would leave no trace. prefix carries its slash.
func RequireDriver(prefix, id string) error {
	if err := ValidateName("driver", id); err != nil {
		return err
	}
	module := prefix + id
	modules, err := os.ReadFile(ModulesFile)
	if err != nil {
		return fmt.Errorf("read %s: %w", ModulesFile, err)
	}
	if !strings.Contains(string(modules), fmt.Sprintf("_ %q", module)) {
		return fmt.Errorf("%s has no %q driver — add\n\t_ %q\nto its import block", ModulesFile, id, module)
	}
	return nil
}
