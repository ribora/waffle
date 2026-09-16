package command

import (
	"bufio"
	"bytes"
	"embed"
	"errors"
	"fmt"
	"go/format"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"text/template"

	"gitlab.com/gorib/waffle/cmd/waffle/internal/pkg/plural"
	"gitlab.com/gorib/waffle/replacements"
)

func New(name, description string, templates embed.FS, funcs template.FuncMap) *Command {
	c := &Command{
		name:        name,
		description: description,
		templates:   templates,
		funcs: Funcs(),
	}
	for n, f := range funcs {
		c.funcs[n] = f
	}
	return c
}

// Exported so a test renders with the scaffolder's functions, not a copy.
func Funcs() template.FuncMap {
	return template.FuncMap{
		"plural": plural.Plural,
		"title":  Title,
		"camel":  Camel,
	}
}

type Command struct {
	name        string
	description string
	operands    string
	templates   embed.FS
	funcs       template.FuncMap
}

// One string, so a bad invocation and --help cannot disagree.
func (c *Command) WithUsage(operands string) *Command {
	c.operands = operands
	return c
}

func (c *Command) Code() string {
	return c.name
}

func (c *Command) Description() string {
	return c.description
}

func (c *Command) Usage() string {
	if c.operands == "" {
		return "usage: waffle " + c.name
	}
	return "usage: waffle " + c.name + " " + c.operands
}

// Called before the first write: UncommentModule needs this file, and reaching
// it last leaves a half-scaffolded domain behind.
func (c *Command) RequireModules() error {
	path := filepath.Join("config", "di", "modules.go")
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("%s is missing — run this from the root of a waffle service", path)
	}
	return nil
}

func (c *Command) Write(sources map[string]string, params any) error {
	type pair struct{ source, target string }

	pairs := make([]pair, 0, len(sources))
	for source, target := range sources {
		pairs = append(pairs, pair{source, target})
	}
	// Map order would shuffle the report between runs, and a report that reshuffles
	// is one nobody reads twice.
	slices.SortFunc(pairs, func(a, b pair) int { return strings.Compare(a.target, b.target) })

	var created, kept []string
	for _, p := range pairs {
		wrote, err := c.write(p.source, p.target, params)
		if err != nil {
			return err
		}
		if wrote {
			created = append(created, p.target)
		} else {
			kept = append(kept, p.target)
		}
	}
	reportCreated(created)
	reportKept(kept)
	return nil
}

// Render is for the templates that do not become a file of their own — a block
// appended to a file somebody else owns.
func (c *Command) Render(source string, params any) ([]byte, error) {
	tpl, err := template.New(filepath.Base(source)).Funcs(c.funcs).ParseFS(c.templates, source)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template %s: %w", source, err)
	}
	var out bytes.Buffer
	if err = tpl.Execute(&out, params); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func (c *Command) write(source, target string, params any) (created bool, err error) {
	// An unreadable target used to count as "already there" and was skipped in
	// silence, which is the one case where skipping is wrong.
	switch _, err := os.Stat(target); {
	case err == nil:
		return false, nil
	case !errors.Is(err, os.ErrNotExist):
		return false, fmt.Errorf("failed to stat %s: %w", target, err)
	}

	tpl, err := template.New(filepath.Base(source)).Funcs(c.funcs).ParseFS(c.templates, source)
	if err != nil {
		return false, fmt.Errorf("failed to parse template %s: %w", source, err)
	}
	var rendered bytes.Buffer
	if err = tpl.Execute(&rendered, params); err != nil {
		return false, err
	}
	// Rendering straight to the file made the template the source of formatting,
	// and two copies of one template had already drifted apart on indentation.
	out := rendered.Bytes()
	if filepath.Ext(target) == ".go" {
		formatted, err := format.Source(out)
		if err != nil {
			return false, fmt.Errorf("%s renders invalid Go: %w", source, err)
		}
		out = formatted
	}
	if err = os.MkdirAll(filepath.Dir(target), os.ModePerm); err != nil {
		return false, fmt.Errorf("failed to create directory %s: %w", filepath.Dir(target), err)
	}
	if err = os.WriteFile(target, out, 0644); err != nil {
		return false, fmt.Errorf("failed to create %s: %w", target, err)
	}
	return true, nil
}

// EnableStorage is the tail every command that writes a repository owes: the
// stored struct carries a go:generate line with no tool behind it and the
// adapter needs a sql.Db, so a command that writes one and stops here leaves a
// project that does not build.
func (c *Command) EnableStorage() error {
	if err := c.EnsureTool("gitlab.com/gorib/sql/cmd/store"); err != nil {
		return err
	}
	if err := c.UncommentModule("gitlab.com/gorib/waffle/config/service/sql"); err != nil {
		return err
	}
	return c.UncommentModule("gitlab.com/gorib/waffle/config/tools/migrate")
}

func (c *Command) UncommentModule(modulePath string) error {
	const file = "config/di/modules.go"
	data, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("uncomment %s: %w", modulePath, err)
	}
	commented := fmt.Sprintf("// _ %q", modulePath)
	active := fmt.Sprintf("_ %q", modulePath)
	enabled := strings.Replace(string(data), commented, active, 1)
	if enabled != string(data) {
		if err := os.WriteFile(file, []byte(enabled), 0644); err != nil {
			return err
		}
		reportAction("enabled module " + modulePath)
	}
	// The import may have been uncommented by hand, and then nothing is replaced
	// here and the variables were never added. Checking is cheap, and only what is
	// missing is ever written.
	return appendEnv(modulePath)
}

//go:embed env
var envFragments embed.FS

// A module that has just been enabled needs its variables in .env, or the service
// stops at startup naming one of them. Only missing keys are added — an existing
// value is never touched, so this is safe to run again.
func appendEnv(modulePath string) error {
	name := strings.TrimPrefix(modulePath, "gitlab.com/gorib/waffle/")
	fragment, err := envFragments.ReadFile(filepath.Join("env", name+".env"))
	if err != nil {
		return nil // the module reads no variables
	}

	const file = ".env"
	current, err := os.ReadFile(file)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read %s: %w", file, err)
	}

	present := map[string]bool{}
	for _, line := range strings.Split(string(current), "\n") {
		if key, ok := assignment(line); ok {
			present[key] = true
		}
	}

	var add, keys []string
	for _, line := range strings.Split(strings.TrimRight(string(fragment), "\n"), "\n") {
		key, ok := assignment(line)
		if ok && present[key] {
			continue
		}
		if ok {
			keys = append(keys, key)
		}
		add = append(add, line)
	}
	if len(keys) == 0 {
		return nil
	}

	body := string(current)
	if body != "" && !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	body += "\n" + strings.Join(add, "\n") + "\n"
	if err := os.WriteFile(file, []byte(body), 0644); err != nil {
		return err
	}
	reportAction("added to .env: " + strings.Join(keys, ", "))
	return nil
}

// A comment may well contain an "=" — "# to override: DAEMON_PORT=9090" is not
// an assignment, and treating it as one appends comment blocks with no variables.
func assignment(line string) (string, bool) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return "", false
	}
	key, _, found := strings.Cut(line, "=")
	if !found {
		return "", false
	}
	return strings.TrimSpace(key), true
}

// EnsureTool adds module via `go get -tool` only if go.mod doesn't already
// list it — safe (and network-free) to call on every scaffold.
func (c *Command) EnsureTool(module string) error {
	root, err := findModuleRoot()
	if err != nil {
		return err
	}
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return fmt.Errorf("failed to read go.mod: %w", err)
	}
	if strings.Contains(string(data), module) {
		return nil
	}
	if info, err := exec.Command("go", "get", "-tool", module+"@latest").CombinedOutput(); err != nil {
		return fmt.Errorf("go get -tool %s: %v: %s", module, err, string(info))
	}
	reportAction("added tool " + module)
	return nil
}

func findModuleRoot() (string, error) {
	return replacements.ModuleRoot()
}

func (c *Command) FindProjectName() (string, error) {
	root, err := findModuleRoot()
	if err != nil {
		return "", err
	}
	goMod, err := os.OpenFile(filepath.Join(root, "go.mod"), os.O_RDONLY, os.ModePerm)
	if err != nil {
		return "", fmt.Errorf("failed to open go.mod file: %w", err)
	}
	//nolint:errcheck
	defer goMod.Close()

	sc := bufio.NewScanner(goMod)
	for sc.Scan() {
		line := sc.Text()
		if pkg, found := strings.CutPrefix(line, "module "); found {
			return pkg, nil
		}
	}
	if err := sc.Err(); err != nil {
		return "", fmt.Errorf("failed to read go.mod file: %w", err)
	}
	return "", errors.New("go.mod is invalid")
}

// ModuleRoot is the directory holding go.mod, for a command that reads or writes
// a file at the root rather than one relative to the working directory.
func ModuleRoot() (string, error) {
	return findModuleRoot()
}
