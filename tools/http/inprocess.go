package http

import (
	"context"
	nethttp "net/http"
	"net/http/httptest"
	"strings"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/ribora/http/server"

	"github.com/ribora/waffle/tools/tap"
)

// Setup is what the daemon calls too, so the routes and the middleware under
// test are the ones that ship. wrap goes around the whole server, outermost last.
func Serve(mux server.Mux, global Global, routers []server.Router, wrap ...func(nethttp.Handler) nethttp.Handler) (*InProcess, error) {
	if err := Setup(mux, global, routers); err != nil {
		return nil, err
	}
	mux.Use(matched)
	var handler nethttp.Handler = mux
	for _, w := range wrap {
		handler = w(handler)
	}
	inproc := &InProcess{changed: make(chan struct{})}
	srv := httptest.NewServer(inproc.counted(handler))
	inproc.URL, inproc.Client, inproc.server = srv.URL, srv.Client(), srv
	return inproc, nil
}

// The mux matches on a request of its own; a global layer is where the pattern
// is visible again.
func matched(next server.Handler) server.Handler {
	return func(src server.Source) (server.Renderer, error) {
		resp, err := next(src)
		ctx := src.Request().Context()
		if rc := chi.RouteContext(ctx); rc != nil {
			if p := pattern(rc.RoutePatterns); p != "" && p != "/" {
				tap.Route(ctx, p)
			}
		}
		return resp, err
	}
}

func pattern(parts []string) string {
	p := strings.Join(parts, "")
	for strings.Contains(p, "/*/") {
		p = strings.ReplaceAll(p, "/*/", "/")
	}
	return p
}

type InProcess struct {
	URL    string
	Client *nethttp.Client

	server  *httptest.Server
	mu      sync.Mutex
	running int
	changed chan struct{}
}

func (s *InProcess) counted(next nethttp.Handler) nethttp.Handler {
	return nethttp.HandlerFunc(func(w nethttp.ResponseWriter, r *nethttp.Request) {
		s.mu.Lock()
		s.running++
		s.mu.Unlock()
		defer s.finished()
		next.ServeHTTP(w, r)
	})
}

func (s *InProcess) finished() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.running--
	close(s.changed)
	s.changed = make(chan struct{})
}

func (s *InProcess) Close() { s.server.Close() }

func (s *InProcess) Settle(ctx context.Context, open int) error {
	for {
		s.mu.Lock()
		running, changed := s.running, s.changed
		s.mu.Unlock()
		if running <= open {
			return nil
		}
		select {
		case <-changed:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
