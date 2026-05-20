package app

import (
	"context"
	"strings"

	"gitlab.com/gorib/hilton"
	"gitlab.com/gorib/session"
)

// NewContextWithCommand builds the root ctx for a CLI command — session
// describing the invocation, plus the hilton provider for downstream I/O.
// A nil provider is safe.
func NewContextWithCommand(name string, args []string, provider hilton.Provider) context.Context {
	ctx := session.NewContextWithSession(
		context.Background(),
		session.Session{
			Uri:    name,
			Method: "CLI",
			Body:   strings.Join(args, " "),
		},
	)
	return hilton.ContextWithProvider(ctx, provider)
}
