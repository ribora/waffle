package http

import (
	nethttp "net/http"
	"net/http/httptest"

	"gitlab.com/gorib/http/server"
)

// Setup is what the daemon calls too, so the routes and the middleware under
// test are the ones that ship.
func Serve(mux server.Mux, global Global, routers []server.Router) (*InProcess, error) {
	if err := Setup(mux, global, routers); err != nil {
		return nil, err
	}
	srv := httptest.NewServer(mux)
	return &InProcess{URL: srv.URL, Client: srv.Client(), server: srv}, nil
}

type InProcess struct {
	URL    string
	Client *nethttp.Client

	server *httptest.Server
}

func (s *InProcess) Close() { s.server.Close() }
