package command

func New(name, description string) *Command {
	return &Command{name: name, description: description}
}

type Command struct {
	name        string
	description string
	operands    string
}

// One string, so a bad invocation and --help cannot disagree.
func (c *Command) WithUsage(operands string) *Command {
	c.operands = operands
	return c
}

func (c *Command) Code() string {
	return c.name
}

func (c *Command) Description() string {
	return c.description
}

func (c *Command) Usage() string {
	if c.operands == "" {
		return "usage: waffle " + c.name
	}
	return "usage: waffle " + c.name + " " + c.operands
}
