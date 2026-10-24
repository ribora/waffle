package main

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// Documentation drifts silently: nothing refuses to build when a page names a
// router that was replaced or a command that was renamed. These checks are the
// missing failure.

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func markdownFiles(t *testing.T) []string {
	t.Helper()
	root := repoRoot(t)

	var files []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".md") {
			return err
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func read(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

var relativeLink = regexp.MustCompile(`\]\((?:\./)?([^):#]+\.md)(#[^)]*)?\)`)

func TestDocsLinksResolve(t *testing.T) {
	root := repoRoot(t)
	for _, file := range markdownFiles(t) {
		for _, m := range relativeLink.FindAllStringSubmatch(read(t, file), -1) {
			if strings.HasPrefix(m[1], "http") {
				continue
			}
			target := filepath.Join(filepath.Dir(file), m[1])
			if _, err := os.Stat(target); err != nil {
				rel, _ := filepath.Rel(root, file)
				t.Errorf("%s links to %s, which does not exist", rel, m[1])
			}
		}
	}
}

func TestReadmeListsEveryPage(t *testing.T) {
	root := repoRoot(t)
	index := read(t, filepath.Join(root, "docs/README.md"))

	listed := map[string]bool{}
	for _, m := range regexp.MustCompile(`\]\(([^):#]+\.md)(?:#[^)]*)?\)`).FindAllStringSubmatch(index, -1) {
		listed[m[1]] = true
	}

	for _, file := range markdownFiles(t) {
		rel, err := filepath.Rel(filepath.Join(root, "docs"), file)
		if err != nil || strings.HasPrefix(rel, "..") || rel == "README.md" {
			continue
		}
		if !listed[filepath.ToSlash(rel)] {
			t.Errorf("docs/%s exists but docs/README.md does not link it — a page nobody can find", rel)
		}
	}
}

// The index reaches every page; this is the way back. Without it a reader who
// arrives from a deep link has no route to anything else.
func TestEveryPageLinksBackToTheIndex(t *testing.T) {
	root := repoRoot(t)
	for _, file := range markdownFiles(t) {
		rel, err := filepath.Rel(filepath.Join(root, "docs"), file)
		if err != nil || strings.HasPrefix(rel, "..") || rel == "README.md" {
			continue
		}
		if !strings.Contains(read(t, file), "[Docs](") {
			t.Errorf("docs/%s has no breadcrumb back to the index", rel)
		}
	}
}

// TestDocsLinksResolve stops at the file. An anchor that no heading produces is
// the other half, and the one that had actually rotted: `#waffle.yaml` loses its
// dot in the slug, and a heading renamed leaves every link to it pointing at the
// top of the page instead of failing.
var headingLine = regexp.MustCompile(`(?m)^#{1,6}\s+(.*)$`)

// GitHub and GitLab both lowercase, drop all but word characters, spaces and
// hyphens, then replace each remaining space with a hyphen.
var notSlug = regexp.MustCompile(`[^\p{L}\p{N}_\s-]`)

func anchors(t *testing.T, path string) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	for _, m := range headingLine.FindAllStringSubmatch(read(t, path), -1) {
		slug := notSlug.ReplaceAllString(strings.ToLower(strings.TrimSpace(m[1])), "")
		out[strings.ReplaceAll(slug, " ", "-")] = true
	}
	return out
}

var anchoredLink = regexp.MustCompile(`\]\((?:\./)?([^):#]*\.md)?#([^)\s]+)\)`)

func TestDocsAnchorsResolve(t *testing.T) {
	root := repoRoot(t)
	known := map[string]map[string]bool{}

	for _, file := range markdownFiles(t) {
		for _, m := range anchoredLink.FindAllStringSubmatch(read(t, file), -1) {
			target := file
			if m[1] != "" {
				target = filepath.Join(filepath.Dir(file), m[1])
				if _, err := os.Stat(target); err != nil {
					continue // TestDocsLinksResolve reports the missing file
				}
			}
			if _, done := known[target]; !done {
				known[target] = anchors(t, target)
			}
			if !known[target][m[2]] {
				rel, _ := filepath.Rel(root, file)
				where, _ := filepath.Rel(root, target)
				t.Errorf("%s links to #%s in %s, which has no such heading", rel, m[2], where)
			}
		}
	}
}

const agentsTemplate = "internal/commands/agents/templates/waffle.md.tpl"

func TestAgentsTemplateRuleNumbering(t *testing.T) {
	body := read(t, filepath.Join(repoRoot(t), "cmd/waffle", agentsTemplate))

	var rules []int
	for _, m := range regexp.MustCompile(`(?m)^## Rule #(\d+)`).FindAllStringSubmatch(body, -1) {
		n, _ := strconv.Atoi(m[1])
		rules = append(rules, n)
	}
	if len(rules) == 0 {
		t.Fatal("no rules found — the template lost its structure")
	}
	// The numbers are identifiers, not a reading order, and #11 left the framework
	// with its number. What still has to hold is that every other number from 1 to N
	// appears exactly once, so an edit cannot lose a rule or leave two with the same
	// name.
	const retired = 11
	seen := make(map[int]bool, len(rules))
	for _, n := range rules {
		if seen[n] {
			t.Errorf("Rule #%d appears twice", n)
		}
		seen[n] = true
	}
	if seen[retired] {
		t.Errorf("Rule #%d is back in the template, but its number is retired", retired)
	}
	for n := 1; n <= len(rules)+1; n++ {
		if n != retired && !seen[n] {
			t.Errorf("Rule #%d is missing, but the template carries %d rules", n, len(rules))
		}
	}

	for _, m := range regexp.MustCompile(`see Rule #(\d+)`).FindAllStringSubmatch(body, -1) {
		n, _ := strconv.Atoi(m[1])
		if !slices.Contains(rules, n) {
			t.Errorf("a cross-reference points at Rule #%d, which does not exist", n)
		}
	}
}

// `waffle <code>` or `./waffle <code>` inside backticks, so prose about "waffle
// conventions" is ignored.
var citedCommand = regexp.MustCompile("`(?:\\./)?waffle ([a-z][a-z:]*)")

func TestDocsCiteRealCommands(t *testing.T) {
	root := repoRoot(t)
	cmds := newCommands()

	files := append(markdownFiles(t), filepath.Join(root, "cmd/waffle", agentsTemplate))
	for _, file := range files {
		seen := map[string]bool{}
		for _, m := range citedCommand.FindAllStringSubmatch(read(t, file), -1) {
			code := m[1]
			if seen[code] || cmds.Get(code) != nil {
				continue
			}
			seen[code] = true
			rel, _ := filepath.Rel(root, file)
			t.Errorf("%s documents `waffle %s`, which is not a registered command (have: %s)",
				rel, code, strings.Join(registered(cmds), ", "))
		}
	}
}

// The other direction, which is how the table drifted: three commands existed
// and were listed nowhere an agent reads first. Rule #1 is presented as the
// whole toolchain, so a command missing from it reads as a command that does not
// exist — and the work gets done by hand instead.
func TestAgentsTemplateListsEveryCommand(t *testing.T) {
	body := read(t, filepath.Join(repoRoot(t), "cmd/waffle", agentsTemplate))

	cited := map[string]bool{}
	for _, m := range citedCommand.FindAllStringSubmatch(body, -1) {
		cited[m[1]] = true
	}
	cmds := newCommands()
	for code := range cmds.All() {
		if !cited[code] {
			t.Errorf("`waffle %s` is registered but waffle.md never mentions it — "+
				"add it to Rule #1's table", code)
		}
	}
}

// The check above stops at the command code, so the operands after it drifted.
var (
	citedInvocation = regexp.MustCompile("`(?:\\./)?waffle ([a-z][a-z:]*)([^`]*)`")
	// A whole operand: `domain/<x>/adapter/dto` is a `<path>`, not a slot `<x>`.
	// Optional ones too: `[routing-key]` outlived the operand by that name because
	// only the angle brackets were read.
	placeholder = regexp.MustCompile(`^(<[a-z][a-z-]*>|\[[a-z][a-z-]*\])$`)
)

func TestDocsNameOperandsTheWayTheCommandsDo(t *testing.T) {
	root := repoRoot(t)
	cmds := newCommands()

	files := append(markdownFiles(t), filepath.Join(root, "cmd/waffle", agentsTemplate))
	for _, file := range files {
		seen := map[string]bool{}
		for _, m := range citedInvocation.FindAllStringSubmatch(read(t, file), -1) {
			cmd := cmds.Get(m[1])
			// An unknown command is TestDocsCiteRealCommands' to report.
			if cmd == nil || seen[m[0]] {
				continue
			}
			for _, name := range strings.Fields(m[2]) {
				if !placeholder.MatchString(name) || strings.Contains(cmd.Usage(), name) {
					continue
				}
				seen[m[0]] = true
				rel, _ := filepath.Rel(root, file)
				t.Errorf("%s documents `waffle %s%s`, but the command calls its operands %q",
					rel, m[1], m[2], operands(cmd))
				break
			}
		}
	}
}

// A documented invocation that leaves out a flag the command refuses to run
// without is worse than a wrong operand name: it is copied, and it fails. Only
// invocations that carry operands are held to this — `waffle make:outbox` in a
// sentence is a reference to the command, not a line to run.
func TestDocsCarryTheFlagsTheCommandsRequire(t *testing.T) {
	root := repoRoot(t)
	cmds := newCommands()

	files := append(markdownFiles(t), filepath.Join(root, "cmd/waffle", agentsTemplate))
	for _, file := range files {
		seen := map[string]bool{}
		for _, m := range citedInvocation.FindAllStringSubmatch(read(t, file), -1) {
			cmd := cmds.Get(m[1])
			if cmd == nil || seen[m[0]] || strings.TrimSpace(m[2]) == "" {
				continue
			}
			for _, flag := range requiredFlags(cmd) {
				if strings.Contains(m[2], flag) {
					continue
				}
				seen[m[0]] = true
				rel, _ := filepath.Rel(root, file)
				t.Errorf("%s documents `waffle %s%s`, which %s refuses without %s",
					rel, m[1], m[2], cmd.Code(), flag)
				break
			}
		}
	}
}

// A flag the usage does not wrap in brackets.
func requiredFlags(cmd Command) []string {
	var flags []string
	for _, token := range strings.Fields(operands(cmd)) {
		if strings.HasPrefix(token, "--") {
			flags = append(flags, token)
		}
	}
	return flags
}

// Usage() carries examples on the lines after the first.
func operands(cmd Command) string {
	first, _, _ := strings.Cut(cmd.Usage(), "\n")
	return strings.TrimPrefix(first, "usage: waffle "+cmd.Code()+" ")
}

func registered(cmds commands) []string {
	var codes []string
	for code := range cmds.All() {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	return codes
}

// A module's environment is what its own package reads plus what it pulls in by
// blank import — the same rule diogen follows for registrations. running-locally.md
// is the only place that answers "why won't it start", so it has to match.

var (
	needValue    = regexp.MustCompile(`env\.NeedValue(?:\[[^\]]+\])?\(\s*"([A-Z0-9_]+)"`)
	optValue     = regexp.MustCompile(`env\.Value(?:\[[^\]]+\])?\(\s*"([A-Z0-9_]+)"`)
	configImport = regexp.MustCompile(`_ "gitlab\.com/gorib/waffle/(config/[a-z0-9/]+)"`)
	parenthesis  = regexp.MustCompile(`\([^)]*\)`)
	backticked   = regexp.MustCompile("`([A-Z][A-Z0-9_]*)`")
	docModule    = regexp.MustCompile("`(config/[a-z0-9/]+)`")
)

type moduleEnv struct {
	required map[string]bool
	optional map[string]bool
	includes []string
}

func scanConfigPackages(t *testing.T) map[string]*moduleEnv {
	t.Helper()
	root := repoRoot(t)

	modules := map[string]*moduleEnv{}
	err := filepath.Walk(filepath.Join(root, "config"), func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") {
			return err
		}
		rel, err := filepath.Rel(root, filepath.Dir(path))
		if err != nil {
			return err
		}
		mod, known := modules[rel]
		if !known {
			mod = &moduleEnv{required: map[string]bool{}, optional: map[string]bool{}}
			modules[rel] = mod
		}
		body := read(t, path)
		for _, m := range needValue.FindAllStringSubmatch(body, -1) {
			mod.required[m[1]] = true
		}
		for _, m := range optValue.FindAllStringSubmatch(body, -1) {
			mod.optional[m[1]] = true
		}
		for _, m := range configImport.FindAllStringSubmatch(body, -1) {
			mod.includes = append(mod.includes, m[1])
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return modules
}

func (m *moduleEnv) resolve(all map[string]*moduleEnv, seen map[string]bool) (req, opt map[string]bool) {
	req, opt = map[string]bool{}, map[string]bool{}
	for k := range m.required {
		req[k] = true
	}
	for k := range m.optional {
		opt[k] = true
	}
	for _, include := range m.includes {
		if seen[include] {
			continue
		}
		seen[include] = true
		included, known := all[include]
		if !known {
			continue
		}
		ireq, iopt := included.resolve(all, seen)
		for k := range ireq {
			req[k] = true
		}
		for k := range iopt {
			opt[k] = true
		}
	}
	return req, opt
}

func cellVars(cell string) map[string]bool {
	out := map[string]bool{}
	for _, m := range backticked.FindAllStringSubmatch(parenthesis.ReplaceAllString(cell, ""), -1) {
		out[m[1]] = true
	}
	return out
}

func TestRunningLocallyMatchesTheCode(t *testing.T) {
	root := repoRoot(t)
	modules := scanConfigPackages(t)

	rows := 0
	for _, line := range strings.Split(read(t, filepath.Join(root, "docs/reference/running-locally.md")), "\n") {
		cells := strings.Split(line, "|")
		if len(cells) < 6 {
			continue
		}
		listed := docModule.FindAllStringSubmatch(cells[1], -1)
		if len(listed) == 0 {
			continue
		}
		rows++

		documentedRequired := cellVars(cells[3])
		documentedOptional := cellVars(cells[4])

		for _, m := range listed {
			mod, known := modules[m[1]]
			if !known {
				t.Errorf("running-locally.md lists %s, which is not a package under config/", m[1])
				continue
			}
			req, opt := mod.resolve(modules, map[string]bool{})
			compare(t, m[1], "required", req, documentedRequired)
			compare(t, m[1], "optional", opt, documentedOptional)
		}
	}
	if rows == 0 {
		t.Fatal("no module rows found — the table lost its shape")
	}
}

func compare(t *testing.T, module, kind string, inCode, inDocs map[string]bool) {
	t.Helper()
	for name := range inCode {
		if !inDocs[name] {
			t.Errorf("%s reads %s %s, running-locally.md does not list it", module, kind, name)
		}
	}
	for name := range inDocs {
		if !inCode[name] {
			t.Errorf("running-locally.md lists %s as %s for %s, the code does not read it", name, kind, module)
		}
	}
}

// Enabling a module writes its variables into .env, either from an env fragment
// (modules a command turns on) or from the scaffolded .env itself (modules that
// are on from the start). Whichever it is, it has to match what the code reads.

const (
	envFragmentDir = "cmd/waffle/internal/pkg/command/env"
	scaffoldedEnv  = "cmd/waffle/internal/commands/service/templates/env.tpl"
	modulesTpl     = "cmd/waffle/internal/commands/service/templates/config/modules.go.tpl"
)

var (
	activeImport    = regexp.MustCompile(`(?m)^\s*_ "gitlab\.com/gorib/waffle/(config/[a-z0-9/]+)"`)
	commentedImport = regexp.MustCompile(`(?m)^\s*// _ "gitlab\.com/gorib/waffle/(config/[a-z0-9/]+)"`)
	envKey          = regexp.MustCompile(`(?m)^([A-Z][A-Z0-9_]*)=`)
)

func envKeys(body string) map[string]bool {
	out := map[string]bool{}
	for _, m := range envKey.FindAllStringSubmatch(body, -1) {
		out[m[1]] = true
	}
	return out
}

func TestEnvFragmentsMatchTheCode(t *testing.T) {
	root := repoRoot(t)
	modules := scanConfigPackages(t)
	tpl := read(t, filepath.Join(root, modulesTpl))

	// modules that are on from the first build are covered by the scaffolded .env
	fromScaffold := envKeys(read(t, filepath.Join(root, scaffoldedEnv)))
	for _, m := range activeImport.FindAllStringSubmatch(tpl, -1) {
		mod, known := modules[m[1]]
		if !known {
			t.Errorf("modules.go.tpl enables %s, which is not a package under config/", m[1])
			continue
		}
		req, opt := mod.resolve(modules, map[string]bool{})
		for name := range req {
			if !fromScaffold[name] {
				t.Errorf("%s is enabled by default and requires %s, missing from the scaffolded .env", m[1], name)
			}
		}
		for name := range opt {
			if !fromScaffold[name] {
				t.Errorf("%s is enabled by default and reads %s, missing from the scaffolded .env", m[1], name)
			}
		}
	}

	// modules a command turns on are covered by an env fragment
	for _, m := range commentedImport.FindAllStringSubmatch(tpl, -1) {
		mod, known := modules[m[1]]
		if !known {
			t.Errorf("modules.go.tpl offers %s, which is not a package under config/", m[1])
			continue
		}
		req, opt := mod.resolve(modules, map[string]bool{})
		if len(req) == 0 && len(opt) == 0 {
			continue
		}

		path := filepath.Join(root, envFragmentDir, m[1]+".env")
		body, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("%s reads environment variables but has no fragment at %s/%s.env",
				m[1], envFragmentDir, m[1])
			continue
		}
		keys := envKeys(string(body))

		for name := range req {
			if !keys[name] {
				t.Errorf("%s requires %s, its env fragment does not list it", m[1], name)
			}
		}
		for name := range opt {
			if !keys[name] {
				t.Errorf("%s reads %s, its env fragment does not list it", m[1], name)
			}
		}
		for name := range keys {
			if !req[name] && !opt[name] {
				t.Errorf("the env fragment for %s lists %s, the code does not read it", m[1], name)
			}
		}
	}
}

func TestEveryEnvFragmentBelongsToAModule(t *testing.T) {
	root := repoRoot(t)
	modules := scanConfigPackages(t)

	err := filepath.Walk(filepath.Join(root, envFragmentDir), func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".env") {
			return err
		}
		rel, err := filepath.Rel(filepath.Join(root, envFragmentDir), path)
		if err != nil {
			return err
		}
		module := strings.TrimSuffix(rel, ".env")
		if _, known := modules[module]; !known {
			t.Errorf("%s.env has no module — config/%s does not exist",
				module, strings.TrimPrefix(module, "config/"))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
