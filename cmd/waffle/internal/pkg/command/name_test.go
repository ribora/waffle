package command

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The name reaches a Go identifier, so what it refuses is what would not compile.
func TestValidateName(t *testing.T) {
	valid := []string{"user", "order", "order_item", "user2", "u"}
	for _, name := range valid {
		if err := ValidateName("domain", name); err != nil {
			t.Errorf("%q was refused: %v", name, err)
		}
	}

	invalid := map[string]string{
		"empty":         "",
		"flag":          "--help",
		"short flag":    "-h",
		"dash":          "user-profile",
		"dot":           "order.paid",
		"leading digit": "2fa",
		"uppercase":     "User",
		"space":         "user profile",
		"slash":         "../etc",
		"keyword":       "type",
		"keyword 2":     "range",
	}
	for what, name := range invalid {
		if err := ValidateName("domain", name); err == nil {
			t.Errorf("%s (%q) was accepted", what, name)
		}
	}
}

// The name is in the message because the author has to see which of two or three
// operands was wrong.
func TestValidateNameNamesTheOperand(t *testing.T) {
	err := ValidateName("event", "order.paid")
	if err == nil {
		t.Fatal("expected an error")
	}
	for _, want := range []string{"event", "order.paid"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

func TestRejectFlags(t *testing.T) {
	if err := RejectFlags([]string{"user", "order_paid"}); err != nil {
		t.Errorf("positionals were refused: %v", err)
	}
	err := RejectFlags([]string{"user", "--nostorage"})
	if err == nil {
		t.Fatal("a misspelled flag was taken as a positional")
	}
	if !strings.Contains(err.Error(), "--nostorage") {
		t.Errorf("error %q does not name the flag", err)
	}
}

func TestValidateFileName(t *testing.T) {
	for _, name := range []string{"create_users_table", "add-index", "v2.1"} {
		if err := ValidateFileName("description", name); err != nil {
			t.Errorf("%q was refused: %v", name, err)
		}
	}
	for _, name := range []string{"", ".", "..", "a/b", `a\b`, "../../etc/passwd"} {
		if err := ValidateFileName("description", name); err == nil {
			t.Errorf("%q was accepted", name)
		}
	}
}

func TestValidatePath(t *testing.T) {
	for _, p := range []string{"domain/users/cmd/grpc/convert", "pkg/dto", "a"} {
		if err := ValidatePath("path", p); err != nil {
			t.Errorf("%q was refused: %v", p, err)
		}
	}
	for _, p := range []string{"", "/etc", "../outside", "domain/../../etc"} {
		if err := ValidatePath("path", p); err == nil {
			t.Errorf("%q was accepted", p)
		}
	}
}

// The broker owns the grammar, so only the unusable literals are refused.
func TestValidateRoutingKey(t *testing.T) {
	for _, key := range []string{"order.paid", "order.*", "order.#", "order_paid"} {
		if err := ValidateRoutingKey(key); err != nil {
			t.Errorf("%q was refused: %v", key, err)
		}
	}
	for _, key := range []string{"", "order paid", `order"paid`} {
		if err := ValidateRoutingKey(key); err == nil {
			t.Errorf("%q was accepted", key)
		}
	}
}

func TestIsHelp(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"-h"}, {"user", "--help"}} {
		if !IsHelp(args) {
			t.Errorf("%v was not read as a help request", args)
		}
	}
	for _, args := range [][]string{{}, {"user"}, {"--no-storage", "user"}} {
		if IsHelp(args) {
			t.Errorf("%v was read as a help request", args)
		}
	}
}

func TestUsage(t *testing.T) {
	bare := New("make:agents", "", testTemplates, nil)
	if got := bare.Usage(); got != "usage: waffle make:agents" {
		t.Errorf("usage = %q", got)
	}
	withOperands := New("make:domain", "", testTemplates, nil).WithUsage("[--no-storage] <domain>")
	if got := withOperands.Usage(); got != "usage: waffle make:domain [--no-storage] <domain>" {
		t.Errorf("usage = %q", got)
	}
}

// UncommentModule used to be reached last, after eleven files and a go.mod edit.
func TestRequireModules(t *testing.T) {
	c, _ := scaffoldIn(t)

	if err := c.RequireModules(); err == nil {
		t.Error("a project without config/di/modules.go passed the precondition")
	}

	if err := os.MkdirAll(filepath.Join("config", "di"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join("config", "di", "modules.go"), []byte("package di\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := c.RequireModules(); err != nil {
		t.Errorf("err = %v", err)
	}
}
