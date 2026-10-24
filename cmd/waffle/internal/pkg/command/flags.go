package command

import (
	"fmt"
	"strings"
)

// Called before Operands, which would count a flag and its value as operands.
func TakeValues(args []string, flag, takes string) (values, rest []string, err error) {
	for i := 0; i < len(args); i++ {
		var value string
		switch joined, found := strings.CutPrefix(args[i], flag+"="); {
		case found:
			value = joined
		case args[i] != flag:
			rest = append(rest, args[i])
			continue
		case i+1 < len(args):
			i++
			value = args[i]
		}
		if value == "" {
			return nil, nil, fmt.Errorf("%s takes %s", flag, takes)
		}
		values = append(values, value)
	}
	return values, rest, nil
}

func TakeBool(args []string, flag string) (bool, []string) {
	var rest []string
	for _, arg := range args {
		if arg != flag {
			rest = append(rest, arg)
		}
	}
	return len(rest) < len(args), rest
}

func TakeDriver(args []string) (string, []string, error) {
	ids, rest, err := TakeValues(args, "--driver", "the broker module's name, e.g. --driver amqp")
	switch {
	case err != nil:
		return "", nil, err
	case len(ids) > 1:
		return "", nil, fmt.Errorf("--driver is given %d times: a service takes one broker", len(ids))
	case len(ids) == 0:
		return "", rest, nil
	}
	if err := ValidateName("driver", ids[0]); err != nil {
		return "", nil, err
	}
	return ids[0], rest, nil
}
