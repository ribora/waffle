package repository

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const project = "example.com/svc"

// A domain as make:domain leaves it before its own entity is written: the
// package exists, service.go is there, nothing else.
func domainDir(t *testing.T, domain string) {
	t.Helper()
	t.Chdir(t.TempDir())
	for _, dir := range []string{"model", "service", filepath.Join("adapter", "postgres")} {
		if err := os.MkdirAll(filepath.Join("domain", domain, dir), 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join("config", "di"), 0755); err != nil {
		t.Fatal(err)
	}
	body := "package service\n\nimport (\n\t\"" + project + "/domain/" + domain + "/model\"\n)\n\n" +
		"var _ = model." + title(domain) + "{}\n\nfunc New" + title(domain) + "Management() *" + domain + "Management {\n" +
		"\treturn &" + domain + "Management{}\n}\n\ntype " + domain + "Management struct{}\n"
	if err := os.WriteFile(filepath.Join("domain", domain, "service", "service.go"), []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
}

// What an entity of a domain is, as a list of paths. Naming it here rather than
// diffing two runs keeps the assertion readable when the domain and its own
// entity share a name.
func artefacts(domain, entity string) []string {
	return []string{
		filepath.Join("domain", domain, "model", entity+".go"),
		filepath.Join("domain", domain, "adapter", "postgres", entity+".go"),
		filepath.Join("domain", domain, "adapter", "postgres", "stored_"+entity+".go"),
		filepath.Join("config", "di", "di_"+domain+"_"+entity+"_repository.go"),
	}
}

func assertEntity(t *testing.T, domain, entity string) {
	t.Helper()
	for _, path := range artefacts(domain, entity) {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("%s/%s is missing %s", domain, entity, filepath.ToSlash(path))
		}
	}
	body, err := os.ReadFile(filepath.Join("domain", domain, "service", "service.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "type "+title(entity)+"Repository interface") {
		t.Errorf("%s/%s has no interface in service.go:\n%s", domain, entity, body)
	}
}

// The invariant the split exists for: which entity of a domain this is changes
// nothing about what it gets.
func TestEveryEntityGetsTheSameFiles(t *testing.T) {
	domainDir(t, "shipment")
	if err := GenerateEntity("shipment", "shipment", project); err != nil {
		t.Fatalf("the domain's own entity: %v", err)
	}
	if err := GenerateEntity("shipment", "line", project); err != nil {
		t.Fatalf("an entity added later: %v", err)
	}

	assertEntity(t, "shipment", "shipment")
	assertEntity(t, "shipment", "line")
}

// The interface belongs in service.go, beside the management that takes it, and
// not in a file of its own.
func TestTheInterfaceLandsInServiceGo(t *testing.T) {
	domainDir(t, "order")
	if err := GenerateEntity("order", "line", project); err != nil {
		t.Fatal(err)
	}

	body, err := os.ReadFile(filepath.Join("domain", "order", "service", "service.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "type LineRepository interface") {
		t.Fatalf("the interface is not in service.go:\n%s", body)
	}
	// It needs context and criteria, which a domain without a repository had not
	// imported — goimports is what supplies them.
	for _, want := range []string{`"context"`, "gorib/criteria"} {
		if !strings.Contains(string(body), want) {
			t.Errorf("service.go is missing import %s:\n%s", want, body)
		}
	}
	if _, err := os.Stat(filepath.Join("domain", "order", "service", "line_repository.go")); err == nil {
		t.Error("the per-entity interface file is back")
	}
}

// A model a person wrote in another file of the package is the name being
// taken, and writing a second declaration would break the build.
func TestAnExistingModelIsFoundWhereverItIs(t *testing.T) {
	domainDir(t, "order")
	mine := "package model\n\ntype Order struct{}\n\ntype Line struct{ Mine bool }\n"
	if err := os.WriteFile(filepath.Join("domain", "order", "model", "order.go"), []byte(mine), 0644); err != nil {
		t.Fatal(err)
	}

	if err := GenerateEntity("order", "line", project); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join("domain", "order", "model", "line.go")); err == nil {
		t.Fatal("a second declaration of model.Line was written into line.go")
	}
	body, _ := os.ReadFile(filepath.Join("domain", "order", "model", "order.go"))
	if !strings.Contains(string(body), "Mine bool") {
		t.Errorf("the hand-written model was overwritten:\n%s", body)
	}
}
