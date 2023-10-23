package app

import (
	"context"
	"strings"

	"gitlab.com/gorib/session"
)

func NewContextWithCommand(name string, args []string) context.Context {
	return session.NewContextWithSession(
		context.Background(),
		session.Session{
			Uri:    name,
			Method: "CLI",
			Body:   strings.Join(args, " "),
		},
	)
}
