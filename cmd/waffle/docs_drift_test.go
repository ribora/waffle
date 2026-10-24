package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The checks in docs_test.go each came from one page rotting. These came from a
// review that read every page against the code, and they are the findings that
// could be turned into a failure. What they have in common: each was true when
// written and went wrong later, in a place no reader would think to re-check.

// docFiles is markdownFiles plus waffle.md.tpl, which is documentation that
// ships into every service and was outside every check here.
func docFiles(t *testing.T) []string {
	t.Helper()
	return append(markdownFiles(t), filepath.Join(repoRoot(t), "cmd/waffle", agentsTemplate))
}

// A page can reach config/tools/http/init.go, the outbox README or the
// outboxtest package — relative paths GitLab resolves and TestDocsLinksResolve
// does not, because it only looks at .md.
var relativeTarget = regexp.MustCompile(`\]\((\.\.?/[^):#]+)(?:#[^)]*)?\)`)

func TestDocsLinkToFilesThatExist(t *testing.T) {
	root := repoRoot(t)
	for _, file := range docFiles(t) {
		for _, m := range relativeTarget.FindAllStringSubmatch(read(t, file), -1) {
			target := filepath.Join(filepath.Dir(file), m[1])
			if _, err := os.Stat(target); err != nil {
				rel, _ := filepath.Rel(root, file)
				t.Errorf("%s links to %s, which does not exist", rel, m[1])
			}
		}
	}
}

// TestDocsAnchorsResolve reads markdown links. A path in backticks is not one,
// and that is how `overview.md#waffle.yaml` survived in the agents template: the slug
// drops the dot, so the link lands at the top of the page instead of failing.
// The anchor may hold a dot, because that is the failure: `#waffle.yaml` is
// what somebody types and `#waffleyaml` is what the heading produces.
var backtickedAnchor = regexp.MustCompile("`([a-z0-9/.-]+\\.md)#([a-z0-9.-]+)`")

func TestDocsAnchorsInBackticksResolve(t *testing.T) {
	root := repoRoot(t)
	pages := markdownFiles(t)

	for _, file := range docFiles(t) {
		for _, m := range backtickedAnchor.FindAllStringSubmatch(read(t, file), -1) {
			// `cli/overview.md` and `di/overview.md` both end in overview.md, so
			// the citation is matched by its whole path and an ambiguous one is
			// left alone rather than checked against the wrong page.
			var matched []string
			for _, page := range pages {
				if strings.HasSuffix(filepath.ToSlash(page), "/"+m[1]) {
					matched = append(matched, page)
				}
			}
			if len(matched) != 1 {
				continue
			}
			if !anchors(t, matched[0])[m[2]] {
				rel, _ := filepath.Rel(root, file)
				hint := ""
				if slug := strings.ReplaceAll(m[2], ".", ""); slug != m[2] && anchors(t, matched[0])[slug] {
					hint = " — the slug drops the dot, so it is #" + slug
				}
				t.Errorf("%s cites %s#%s, which has no such heading%s", rel, m[1], m[2], hint)
			}
		}
	}
}

// Every gorib repository is on main. A link to /blob/master/ is a 404 that
// nothing else notices, because the host answers it.
var gitlabBlob = regexp.MustCompile(`gitlab\.com/gorib/[a-z]+/-/blob/([a-z]+)/`)

func TestDocsLinkToTheDefaultBranch(t *testing.T) {
	root := repoRoot(t)
	for _, file := range docFiles(t) {
		for _, m := range gitlabBlob.FindAllStringSubmatch(read(t, file), -1) {
			if m[1] == "main" {
				continue
			}
			rel, _ := filepath.Rel(root, file)
			t.Errorf("%s links to the %q branch; gorib repositories are on main", rel, m[1])
		}
	}
}

// running-locally.md is checked against config/**; environment.md is the same
// table for the same variables and was not. They agree today.
func TestEnvironmentReferenceMatchesTheCode(t *testing.T) {
	root := repoRoot(t)
	modules := scanConfigPackages(t)

	documented := map[string]bool{}
	for _, m := range backticked.FindAllStringSubmatch(read(t, filepath.Join(root, "docs/reference/environment.md")), -1) {
		documented[m[1]] = true
	}
	if len(documented) == 0 {
		t.Fatal("no variables found in environment.md — the page lost its shape")
	}

	for name, mod := range modules {
		req, opt := mod.resolve(modules, map[string]bool{})
		for _, set := range []map[string]bool{req, opt} {
			for v := range set {
				if !documented[v] {
					t.Errorf("%s reads %s, environment.md does not document it", name, v)
				}
			}
		}
	}
}

// The scaffolded Makefile is what Rule #2 and half a dozen pages tell people to
// run. `make` was documented as running tests for as long as there was no test
// target to run.
const makefileTpl = "cmd/waffle/internal/commands/service/templates/Makefile.tpl"

var (
	makeTarget = regexp.MustCompile(`(?m)^([a-z.][a-z.-]*):`)
	citedMake  = regexp.MustCompile("`make ([a-z.][a-z.-]*)")
)

func TestDocsCiteRealMakeTargets(t *testing.T) {
	root := repoRoot(t)

	targets := map[string]bool{}
	for _, m := range makeTarget.FindAllStringSubmatch(read(t, filepath.Join(root, makefileTpl)), -1) {
		targets[m[1]] = true
	}
	if len(targets) == 0 {
		t.Fatal("no targets found in Makefile.tpl — the template lost its shape")
	}

	for _, file := range docFiles(t) {
		seen := map[string]bool{}
		for _, m := range citedMake.FindAllStringSubmatch(read(t, file), -1) {
			// `make generate && make audit` — the operand of the first is a
			// target, everything after && is matched on its own.
			name := m[1]
			if targets[name] || seen[name] {
				continue
			}
			seen[name] = true
			rel, _ := filepath.Rel(root, file)
			t.Errorf("%s documents `make %s`, which Makefile.tpl has no target for", rel, name)
		}
	}
}

// `all` is what a bare `make` runs, and whether the component suite is in it
// decides what a green `make` means. Several pages state the answer; this is
// the fact they state.
func TestBareMakeDoesNotRunTheSuite(t *testing.T) {
	body := read(t, filepath.Join(repoRoot(t), makefileTpl))

	all := ""
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "all:") {
			all = line
			break
		}
	}
	if all == "" {
		t.Fatal("Makefile.tpl has no `all` target")
	}
	// If this is ever wanted, it is a deliberate change — and the pages saying a
	// green `make` does not cover the boundaries have to change with it:
	// init-service.md, and Rule #2 and #12 in waffle.md.tpl.
	if strings.Contains(all, "test") {
		t.Errorf("`all` now runs the suite (%q) — update the pages that say a green `make` "+
			"does not cover boundaries.Check", strings.TrimSpace(all))
	}
}
