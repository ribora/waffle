package flows

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func service(t *testing.T, root, name, doc string) {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Join(dir, "tests"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "waffle.yaml"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tests", "flows.yaml"), []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCheckPassesOnWhatItWroteAndFailsOnAPageLeftBehind(t *testing.T) {
	root := t.TempDir()
	service(t, root, "shop", "service: shop\ninputs:\n  - message: http POST /orders\n    branches:\n      - sends: [grpc /stock.S/Reserve]\n        cases: [a]\n")
	service(t, root, "stock", "service: stock\ninputs:\n  - message: grpc /stock.S/Reserve\n    branches:\n      - cases: [b]\n")
	index := filepath.Join(root, "docs", "flows.yaml")

	if err := New().Run(t.Context(), root, index); err != nil {
		t.Fatal(err)
	}
	if err := New().Run(t.Context(), "check", root, index); err != nil {
		t.Fatalf("check right after the write: %v", err)
	}

	page, err := os.ReadFile(filepath.Join(root, "docs", "shop-http-post-orders.md"))
	if err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(root, "docs", "shop-http-delete-orders.md")
	if err := os.WriteFile(stale, page, 0o644); err != nil {
		t.Fatal(err)
	}
	err = New().Run(t.Context(), "check", root, index)
	if err == nil || !strings.Contains(err.Error(), "gone: "+stale) {
		t.Fatalf("check with a page no flow writes: %v", err)
	}
}
