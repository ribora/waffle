package command

import (
	"embed"
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const ModulesFile = "config/di/modules.go"

var MigrationModules = []string{
	"gitlab.com/gorib/waffle/config/service/sql",
	"gitlab.com/gorib/waffle/config/tools/migrate",
}

// Called before the first write with every module the command will enable:
// reaching a missing line last leaves a half-scaffolded domain behind.
func (p *Project) RequireModules(modules ...string) error {
	body, err := os.ReadFile(p.Path(ModulesFile))
	if errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("%s is missing — run this inside a waffle service", ModulesFile)
	} else if err != nil {
		return err
	}
	var missing []string
	for _, module := range modules {
		if !strings.Contains(string(body), importLine(module)) {
			missing = append(missing, module)
		}
	}
	if len(missing) > 0 {
		return noLine(missing...)
	}
	return nil
}

// EnableStorage is the tail every command that writes a repository owes: the
// stored struct carries a go:generate line with no tool behind it and the
// adapter needs a sql.Db, so a command that writes one and stops here leaves a
// project that does not build.
func (p *Project) EnableStorage() error {
	if err := p.EnsureTool("gitlab.com/gorib/sql/cmd/store"); err != nil {
		return err
	}
	return p.EnableMigrations()
}

// EnableMigrations is what a migration runner needs: the runner it writes is a
// command over a sql.Db, and neither exists until these modules are enabled.
func (p *Project) EnableMigrations() error {
	for _, module := range MigrationModules {
		if err := p.EnableModule(module); err != nil {
			return err
		}
	}
	return nil
}

func (p *Project) EnableModule(module string) error {
	body, err := os.ReadFile(p.Path(ModulesFile))
	if err != nil {
		return fmt.Errorf("enable %s: %w", module, err)
	}
	line := importLine(module)
	switch enabled := strings.Replace(string(body), "// "+line, line, 1); {
	case enabled != string(body):
		if err := os.WriteFile(p.Path(ModulesFile), []byte(enabled), 0644); err != nil {
			return err
		}
		reportAction("enabled module " + module)
	case !strings.Contains(string(body), line):
		return noLine(module)
	}
	// The import may have been uncommented by hand, and then nothing is replaced
	// here and the variables were never added. Checking is cheap, and only what is
	// missing is ever written.
	return p.appendEnv(module)
}

func importLine(module string) string {
	return fmt.Sprintf("_ %q", module)
}

func noLine(modules ...string) error {
	lines := make([]string, len(modules))
	for i, module := range modules {
		lines[i] = "\t" + importLine(module)
	}
	return fmt.Errorf("%s has no line for %s — add\n%s\nto its import block",
		ModulesFile, strings.Join(modules, ", "), strings.Join(lines, "\n"))
}

// Parsed, not matched: the scaffolded modules.go ships with commented-out blank
// imports, and a match counts them as active.
func (p *Project) ActiveModules() ([]string, error) {
	file, err := parser.ParseFile(token.NewFileSet(), p.Path(ModulesFile), nil, parser.ImportsOnly|parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	var active []string
	for _, spec := range file.Imports {
		if spec.Name == nil || spec.Name.Name != "_" {
			continue
		}
		if path, err := strconv.Unquote(spec.Path.Value); err == nil {
			active = append(active, path)
		}
	}
	return active, nil
}

//go:embed env
var envFragments embed.FS

func ModuleEnv(name string) []string {
	fragment, err := envFragments.ReadFile("env/config/" + name + ".env")
	if err != nil {
		return nil
	}
	var out []string
	for line := range strings.Lines(string(fragment)) {
		key, _, found := strings.Cut(strings.TrimSpace(line), "=")
		if found && key != "" && !strings.HasPrefix(key, "#") {
			out = append(out, key)
		}
	}
	return out
}

// A module that has just been enabled needs its variables in .env, or the service
// stops at startup naming one of them. Only missing keys are added — an existing
// value is never touched, so this is safe to run again.
func (p *Project) appendEnv(modulePath string) error {
	name := strings.TrimPrefix(modulePath, "gitlab.com/gorib/waffle/")
	fragment, err := envFragments.ReadFile(filepath.Join("env", name+".env"))
	if err != nil {
		return nil // the module reads no variables
	}

	const file = ".env"
	current, err := os.ReadFile(p.Path(file))
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
	if err := os.WriteFile(p.Path(file), []byte(body), 0644); err != nil {
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
