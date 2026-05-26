package app

import (
	"context"
	"strings"

	"gitlab.com/gorib/hilton"
	"gitlab.com/gorib/pry"
	"gitlab.com/gorib/session"
)

// NewContextWithCommand builds the root ctx for a CLI command — session,
// hilton provider, and pry logger for downstream I/O.
func NewContextWithCommand(name string, args []string, provider hilton.Provider, logger pry.Logger) context.Context {
	ctx := session.NewContextWithSession(
		context.Background(),
		session.Session{
			Uri:    name,
			Method: "CLI",
			Body:   strings.Join(args, " "),
		},
	)
	ctx = hilton.ContextWithProvider(ctx, provider)
	return pry.ContextWithLogger(ctx, logger)
}
