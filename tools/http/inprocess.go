package http

import (
	nethttp "net/http"
	"net/http/httptest"

	"gitlab.com/gorib/http/server"
)

// Router.Setup is what the daemon calls too, so the routes and the middleware
// under test are the ones that ship.
func Serve(mux server.Mux, router server.Router) *InProcess {
	router.Setup(mux)
	srv := httptest.NewServer(mux)
	return &InProcess{URL: srv.URL, Client: srv.Client(), server: srv}
}

type InProcess struct {
	URL    string
	Client *nethttp.Client

	server *httptest.Server
}

func (s *InProcess) Close() { s.server.Close() }
