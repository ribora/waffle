package main

import (
	"regexp"
	"slices"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"
)

// The checks beside this one verify what a page *refers to* — a link, a command
// name, an environment variable. This one verifies what a page *shows*: the
// symbols in its Go blocks. A page saying pry.New takes a level string, or
// naming a constructor the package stopped exporting, is the drift nothing else
// here catches.
//
// Existence only. A signature that changed while the name stayed is past what
// this can see without compiling the snippets, and most of them are fragments.

// A name in a doc block is matched against every package it could plausibly be.
// Two of them are genuinely ambiguous — `server` is both transports, `sql` is
// ours and the standard library's — and a symbol found in either is accepted:
// the point is to catch what exists nowhere, not to police which import a
// fragment meant.
var docPackages = map[string][]string{
	"amqp":        {"gitlab.com/gorib/amqp"},
	"app":         {"gitlab.com/gorib/waffle/app"},
	"client":      {"gitlab.com/gorib/http/client", "gitlab.com/gorib/grpc/client"},
	"criteria":    {"gitlab.com/gorib/criteria"},
	"di":          {"gitlab.com/gorib/di"},
	"elastictest": {"gitlab.com/gorib/elastic/elastictest"},
	"env":         {"gitlab.com/gorib/env"},
	"failure":     {"gitlab.com/gorib/failure"},
	"hilton":      {"gitlab.com/gorib/hilton"},
	"middleware":  {"gitlab.com/gorib/http/server/middleware"},
	"openapi":     {"gitlab.com/gorib/http/openapi"},
	"openapitest": {"gitlab.com/gorib/http/openapi/openapitest"},
	"pry":         {"gitlab.com/gorib/pry"},
	"redistest":   {"gitlab.com/gorib/redis/redistest"},
	"scan":        {"gitlab.com/gorib/whisk/scan"},
	"server":      {"gitlab.com/gorib/http/server", "gitlab.com/gorib/grpc/server"},
	"session":     {"gitlab.com/gorib/session"},
	"sql":         {"gitlab.com/gorib/sql", "database/sql"},
	"sqltest":     {"gitlab.com/gorib/sql/sqltest"},
	"validate":    {"gitlab.com/gorib/whisk/validate"},
}

var (
	goBlock = regexp.MustCompile("(?s)```go\\n(.*?)```")
	// The leading class keeps a field chain out: w.app.Consumer is not the app
	// package, and neither is anything else reached through a dot.
	qualified = regexp.MustCompile(`(^|[^\w.])([a-z][a-zA-Z0-9]*)\.([A-Z][A-Za-z0-9]*)\b`)
)

func exportedByPackage(t *testing.T) map[string]map[string]bool {
	t.Helper()
	paths := map[string]bool{}
	for _, candidates := range docPackages {
		for _, path := range candidates {
			paths[path] = true
		}
	}
	loaded, err := packages.Load(&packages.Config{
		Mode: packages.NeedName | packages.NeedTypes,
		Dir:  repoRoot(t),
	}, slices.Sorted(maps(paths))...)
	if err != nil {
		t.Fatalf("loading the packages the docs name: %v", err)
	}

	out := map[string]map[string]bool{}
	for _, pkg := range loaded {
		if pkg.Types == nil {
			t.Fatalf("%s did not load; the docs cannot be checked against it", pkg.PkgPath)
		}
		names := map[string]bool{}
		for _, name := range pkg.Types.Scope().Names() {
			names[name] = true
		}
		out[pkg.PkgPath] = names
	}
	return out
}

func maps(set map[string]bool) func(func(string) bool) {
	return func(yield func(string) bool) {
		for key := range set {
			if !yield(key) {
				return
			}
		}
	}
}

func TestDocsShowRealSymbols(t *testing.T) {
	exported := exportedByPackage(t)
	root := repoRoot(t)

	checked := 0
	for _, path := range markdownFiles(t) {
		rel := strings.TrimPrefix(path, root+"/")
		for _, block := range goBlock.FindAllStringSubmatch(read(t, path), -1) {
			for _, use := range qualified.FindAllStringSubmatch(block[1], -1) {
				name, symbol := use[2], use[3]
				candidates, known := docPackages[name]
				if !known || shadowed(block[1], name) {
					continue
				}
				checked++
				if !exportedBy(exported, candidates, symbol) {
					t.Errorf("%s shows %s.%s, which %s does not export",
						rel, name, symbol, strings.Join(candidates, " or "))
				}
			}
		}
	}

	// Without this the test passes just as well when the block or the symbol
	// pattern stops matching anything at all.
	if checked < 100 {
		t.Fatalf("only %d symbols were checked — the extraction is broken, not the docs", checked)
	}
	t.Logf("checked %d symbol uses", checked)
}

// A block that declares its own `di` or names its package `scribus` is not
// talking about ours, and a snippet is free to pick whatever name it likes.
func shadowed(block, name string) bool {
	for _, form := range []string{
		"package " + name,
		name + " :=",
		name + ", err :=",
		name + ", _ :=",
		"var " + name + " ",
		"(" + name + " *",
	} {
		if strings.Contains(block, form) {
			return true
		}
	}
	return false
}

func exportedBy(exported map[string]map[string]bool, candidates []string, name string) bool {
	for _, path := range candidates {
		if exported[path][name] {
			return true
		}
	}
	return false
}
