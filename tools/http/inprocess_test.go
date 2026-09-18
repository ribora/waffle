package http

import (
	"context"
	"encoding/json"
	"errors"
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

type widgetRouter struct{}

func (widgetRouter) Setup(mux server.Mux) {
	mux.Post("/widgets", server.Bind(
		func(context.Context, *probeReq) (*server.Response[probeReq], error) {
			return server.OK(&probeReq{Name: "widget"}), nil
		},
	))
}

func serve(t *testing.T, routers ...server.Router) *InProcess {
	t.Helper()
	inproc, err := Serve(server.New(), routers)
	if err != nil {
		t.Fatalf("serve: %v", err)
	}
	t.Cleanup(inproc.Close)
	return inproc
}

func post(t *testing.T, inproc *InProcess, path string) probeReq {
	t.Helper()
	response, err := inproc.Client.Post(inproc.URL+path, "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("post %s: %v", path, err)
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != nethttp.StatusOK {
		t.Fatalf("status for %s: %s", path, response.Status)
	}
	var body probeReq
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("body: %v", err)
	}
	return body
}

func TestServeRoutesThroughTheContainersRouter(t *testing.T) {
	if got := post(t, serve(t, echoRouter{}), "/things"); got.Name != "served" {
		t.Fatalf("the handler did not answer: %+v", got)
	}
}

func TestServeJoinsEveryRouterIntoOneServer(t *testing.T) {
	inproc := serve(t, echoRouter{}, widgetRouter{})

	if got := post(t, inproc, "/things"); got.Name != "served" {
		t.Errorf("the first router did not answer: %+v", got)
	}
	if got := post(t, inproc, "/widgets"); got.Name != "widget" {
		t.Errorf("the second router did not answer: %+v", got)
	}
}

func TestServeNamesBothRoutersOnACollision(t *testing.T) {
	_, err := Serve(server.New(), []server.Router{echoRouter{}, probeRouter{}})

	if err == nil {
		t.Fatal("two routers claimed one route and the server came up")
	}
	for _, want := range []string{"POST /things", "echoRouter", "probeRouter"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the error does not name %s: %v", want, err)
		}
	}
}

func TestServeRefusesAnEmptyCollection(t *testing.T) {
	if _, err := Serve(server.New(), nil); !errors.Is(err, ErrNoRouters) {
		t.Fatalf("serve: %v", err)
	}
}

// A route the router did not register has to 404 rather than reach a handler:
// the mux under test is the one the daemon builds, not a permissive stand-in.
func TestServeAnswersAnUnknownRoute(t *testing.T) {
	inproc := serve(t, echoRouter{})

	response, err := inproc.Client.Get(inproc.URL + "/absent")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != nethttp.StatusNotFound {
		t.Fatalf("status: %s", response.Status)
	}
}
