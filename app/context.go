package app

import (
	"context"
	"strings"

	"github.com/ribora/hilton"
	"github.com/ribora/pry"
	"github.com/ribora/session"
)

type argsKey struct{}

// args is shaped [<command-name>, <rest>...] — same form (*getopt.Set).Getopt expects.
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
	ctx = context.WithValue(ctx, argsKey{}, args)
	return pry.ContextWithLogger(ctx, logger)
}

func Args(ctx context.Context) []string {
	if v, ok := ctx.Value(argsKey{}).([]string); ok {
		return v
	}
	return nil
}

func WithArgs(ctx context.Context, args []string) context.Context {
	return context.WithValue(ctx, argsKey{}, args)
}
