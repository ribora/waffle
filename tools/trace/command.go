package trace

import (
	"context"

	"gitlab.com/gorib/waffle/app"
)

// A daemon serves inputs rather than being one, and keeps its own type.
func Command(c app.Command) app.Command {
	if _, daemon := c.(app.Daemon); daemon || c == nil {
		return c
	}
	return command{c}
}

type command struct{ app.Command }

func (c command) Run(ctx context.Context) error {
	if !on.Load() {
		return c.Command.Run(ctx)
	}
	began := seq.Add(1)
	err := c.Command.Run(ctx)
	r := Record{Seq: began, Dir: "in", Kind: "command", Name: c.Name()}
	if err != nil {
		r.Error = refusal(err)
	}
	write(r)
	return err
}
