package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"text/template"
)

func renderTo(t *testing.T, source string, params map[string]string, target string) {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(repoRoot(t), "cmd", "waffle", "internal", "commands", source))
	if err != nil {
		t.Fatal(err)
	}
	tpl, err := template.New(filepath.Base(source)).Funcs(funcs()).Parse(string(body))
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := tpl.Execute(&out, params); err != nil {
		t.Fatal(err)
	}
	writeFile(t, target, out.Bytes())
}

func writeFile(t *testing.T, target string, body []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, body, 0o644); err != nil {
		t.Fatal(err)
	}
}

func scaffoldedApi(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), []byte("module catalog\n"))
	renderTo(t, filepath.Join("service", "templates", "api", "init.go.tpl"),
		map[string]string{"Service": "catalog"}, filepath.Join(dir, "api", "init.go"))
	return dir
}

func goGenerate(t *testing.T, dir string) ([]byte, error) {
	t.Helper()
	cmd := exec.Command("go", "generate", "./api/...")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off")
	return cmd.CombinedOutput()
}

func TestProtoStepWritesTheStubs(t *testing.T) {
	dir := scaffoldedApi(t)
	renderTo(t, filepath.Join("grpc", "templates", "api", "service", "proto", "entity", "service.proto.tpl"),
		map[string]string{"Domain": "user"}, filepath.Join(dir, "api", "proto", "user", "service.proto"))

	if output, err := goGenerate(t, dir); err != nil {
		t.Fatalf("%v\n%s", err, output)
	}
	for _, stub := range []string{"service.pb.go", "service_grpc.pb.go"} {
		if _, err := os.Stat(filepath.Join(dir, "api", "pb", "user", stub)); err != nil {
			t.Error(err)
		}
	}
}

func TestProtoStepFailsWhenProtocDoes(t *testing.T) {
	dir := scaffoldedApi(t)
	writeFile(t, filepath.Join(dir, "api", "proto", "user", "service.proto"), []byte("not a proto\n"))

	if output, err := goGenerate(t, dir); err == nil {
		t.Fatalf("a proto protoc refuses passed:\n%s", output)
	}
}

func TestProtoStepWithoutAProtoHasNothingToDo(t *testing.T) {
	if output, err := goGenerate(t, scaffoldedApi(t)); err != nil {
		t.Fatalf("%v\n%s", err, output)
	}
}
