package surface

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestExtractReadsWhatCrossesTheEdge(t *testing.T) {
	got, err := Extract(filepath.Join("testdata", "cases", "shop"))
	if err != nil {
		t.Fatal(err)
	}
	want := Surface{
		Version:   Version,
		Service:   "shop",
		Publishes: []string{"orders.canceled", "orders.created", "orders.placed", "orders.updated", "refunds.issued"},
		Consumes:  []string{"payments.refunded", "payments.succeeded", "stock.#"},
		Serves: []string{"/order.OrderService/Place", "/refund.RefundService/Issue",
			"DELETE /admin/users/{userId}", "GET /admin/users/{userId}", "GET /health", "POST /customers/password"},
		Calls: []string{"/audit.AuditService/Record", "/stock.StockService/Reserve",
			"GET /api/documents/{}", "GET external", "POST /api/render/text"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got  %+v\nwant %+v", got, want)
	}
}

// merchandize's adapter calls an interface invoice's service satisfies, but the
// container never hands it one: a shape that fits is not a link.
func TestExtractLinksDomainsTheContainerWires(t *testing.T) {
	got, err := Extract(filepath.Join("testdata", "cases", "monolith"))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"rent → invoice: Invoices.Issue"}
	if !reflect.DeepEqual(got.Domains, want) {
		t.Errorf("got %v, want %v", got.Domains, want)
	}
}

func TestExtractRefusesAKeyItCannotTell(t *testing.T) {
	dir := t.TempDir()
	outbox, err := filepath.Abs(filepath.Join("testdata", "cases", "shop", "fake", "waffle"))
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dir, "go.mod"), "module example.com/lost\n\ngo 1.27.0\n\n"+
		"require gitlab.com/gorib/waffle v0.0.0\n\nreplace gitlab.com/gorib/waffle => "+outbox+"\n")
	write(t, filepath.Join(dir, "lost.go"), `package lost

import (
	"context"

	"gitlab.com/gorib/waffle/tools/outbox"
)

func key() string { return "anything" }

func send(ctx context.Context, p outbox.Publisher) error { return p.Enqueue(ctx, key(), nil) }
`)
	_, err = Extract(dir)
	if err == nil || !strings.Contains(err.Error(), "lost.go:11: publishes a name that is not a constant") {
		t.Fatalf("got %v, want a refusal naming lost.go:11", err)
	}
}

func TestReadRefusesANewerFormat(t *testing.T) {
	file := filepath.Join(t.TempDir(), "surface.yaml")
	write(t, file, "version: 2\nservice: shop\n")
	if _, err := Read(file); err == nil || !strings.Contains(err.Error(), "newer than this waffle reads") {
		t.Fatalf("got %v, want a refusal of version 2", err)
	}
}

func TestEncodeReadsBack(t *testing.T) {
	s := Surface{Version: Version, Service: "shop", Publishes: []string{"a.b"}, Calls: []string{"/x.Y/Z"}}
	file := filepath.Join(t.TempDir(), "surface.yaml")
	write(t, file, string(s.Encode()))
	got, err := Read(file)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, s) {
		t.Errorf("got %+v, want %+v", got, s)
	}
}

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
