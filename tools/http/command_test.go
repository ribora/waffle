package http

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.com/gorib/hilton"
	"gitlab.com/gorib/http/model"
	"gitlab.com/gorib/http/server"
	"gitlab.com/gorib/pry"
	"gitlab.com/gorib/waffle/app"
	"gitlab.com/gorib/whisk/scan"
)

// Parse is whisk's to generate; by hand here so the test needs no codegen step.
type probeReq struct {
	Name string `json:"name"`
}

func (p *probeReq) Parse(scan.Source) error { return nil }

type probeRouter struct{}

func (probeRouter) Setup(mux server.Mux) {
	mux.Post("/things", server.Bind(
		func(ctx context.Context, req *probeReq) (*server.Response[probeReq], error) {
			return server.OK(req), nil
		},
	))
}

// The command is the platform's only caller of Reconcile.
func documentFrom(t *testing.T) string {
	t.Helper()

	logger := pry.Noop()
	path := filepath.Join(t.TempDir(), "openapi.yaml")

	cmd := NewHttpCommand(
		server.New(),
		probeRouter{},
		app.NewCommand("http", "test"),
		nil,
		logger,
	)

	// The command name leads, as it does when app.Run hands the rest to getopt.
	ctx := app.NewContextWithCommand("http", []string{"http", "--openapi", path}, hilton.Noop(), logger)
	if err := cmd.Run(ctx); err != nil {
		t.Fatalf("run: %v", err)
	}
	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("no document was written: %v", err)
	}
	return string(out)
}

func TestOpenapiFlagWritesTheDocument(t *testing.T) {
	got := documentFrom(t)
	for _, want := range []string{"openapi: 3.1.0", "/things", model.ContentTypeJson} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q:\n%s", want, got)
		}
	}
}
