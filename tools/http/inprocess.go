package http

import (
	nethttp "net/http"
	"net/http/httptest"
	"strings"

	"github.com/go-chi/chi/v5"
	"gitlab.com/gorib/http/server"

	"gitlab.com/gorib/waffle/tools/tap"
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
	srv := httptest.NewServer(handler)
	return &InProcess{URL: srv.URL, Client: srv.Client(), server: srv}, nil
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

	server *httptest.Server
}

func (s *InProcess) Close() { s.server.Close() }
