package tap

import (
	"context"
	"sync/atomic"
)

type listener func(dir, key string, body []byte)

var listening atomic.Pointer[listener]

// Listen hands f every message from now on: "in" for a delivery, "out" for a
// publication. Nil stops it.
func Listen(f func(dir, key string, body []byte)) {
	if f == nil {
		listening.Store(nil)
		return
	}
	l := listener(f)
	listening.Store(&l)
}

// Message is what a tool calls for each message it moves.
func Message(dir, key string, body []byte) {
	if l := listening.Load(); l != nil {
		(*l)(dir, key, body)
	}
}

type routeKey struct{}

// WithRoute is where Route writes the pattern a router matched for the request
// ctx belongs to.
func WithRoute(ctx context.Context, route *string) context.Context {
	return context.WithValue(ctx, routeKey{}, route)
}

// Route names the request by the pattern a router matched, for whoever asked
// with WithRoute.
func Route(ctx context.Context, pattern string) {
	if route, ok := ctx.Value(routeKey{}).(*string); ok {
		*route = pattern
	}
}
