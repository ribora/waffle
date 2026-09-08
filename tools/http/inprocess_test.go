package http

import (
	"context"
	"encoding/json"
	nethttp "net/http"
	"strings"
	"testing"

	"gitlab.com/gorib/http/server"
)

// Answers with a value of its own: probeReq.Parse is a stub here, so anything
// read off the request would be testing whisk rather than the server.
type echoRouter struct{}

func (echoRouter) Setup(mux server.Mux) {
	mux.Post("/things", server.Bind(
		func(context.Context, *probeReq) (*server.Response[probeReq], error) {
			return server.OK(&probeReq{Name: "served"}), nil
		},
	))
}

func TestServeRoutesThroughTheContainersRouter(t *testing.T) {
	inproc := Serve(server.New(), echoRouter{})
	t.Cleanup(inproc.Close)

	response, err := inproc.Client.Post(inproc.URL+"/things", "application/json",
		strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != nethttp.StatusOK {
		t.Fatalf("status: %s", response.Status)
	}
	var body probeReq
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("body: %v", err)
	}
	if body.Name != "served" {
		t.Fatalf("the handler did not answer: %+v", body)
	}
}

// A route the router did not register has to 404 rather than reach a handler:
// the mux under test is the one the daemon builds, not a permissive stand-in.
func TestServeAnswersAnUnknownRoute(t *testing.T) {
	inproc := Serve(server.New(), echoRouter{})
	t.Cleanup(inproc.Close)

	response, err := inproc.Client.Get(inproc.URL + "/absent")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != nethttp.StatusNotFound {
		t.Fatalf("status: %s", response.Status)
	}
}
