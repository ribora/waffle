package command

import (
	"bufio"
	"embed"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"gitlab.com/gorib/waffle/cmd/waffle/internal/pkg/plural"
)

func New(name, description string, templates embed.FS, funcs template.FuncMap) *Command {
	c := &Command{
		name:        name,
		description: description,
		templates:   templates,
		funcs: template.FuncMap{
			"plural": plural.Plural,
			"title": func(word string) string {
				return strings.ToUpper(word[:1]) + word[1:]
			},
		},
	}
	for n, f := range funcs {
		c.funcs[n] = f
	}
	return c
}

type Command struct {
	name        string
	description string
	templates   embed.FS
	funcs       template.FuncMap
}

func (c *Command) Code() string {
	return c.name
}

func (c *Command) Description() string {
	return c.description
}

func (c *Command) Write(sources map[string]string, params any) error {
	for source, target := range sources {
		if err := c.write(source, target, params); err != nil {
			return err
		}
	}
	return nil
}

func (c *Command) write(source, target string, params any) error {
	tpl, err := template.New(filepath.Base(source)).Funcs(c.funcs).ParseFS(c.templates, source)
	if err != nil {
		return fmt.Errorf("failed to parse template %s: %w", source, err)
	}
	if _, err = os.Stat(target); !errors.Is(err, os.ErrNotExist) {
		return nil
	}
	err = os.MkdirAll(filepath.Dir(target), os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to create directory %s: %w", filepath.Dir(target), err)
	}
	fh, err := os.Create(target)
	if err != nil {
		return fmt.Errorf("failed to create %s: %w", target, err)
	}
	//nolint:errcheck
	defer fh.Close()
	return tpl.Execute(fh, params)
}

func (c *Command) UncommentModule(modulePath string) error {
	const file = "config/di/modules.go"
	data, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("uncomment %s: %w", modulePath, err)
	}
	commented := fmt.Sprintf("// _ %q", modulePath)
	active := fmt.Sprintf("_ %q", modulePath)
	out := strings.Replace(string(data), commented, active, 1)
	if out == string(data) {
		return nil
	}
	if err := os.WriteFile(file, []byte(out), 0644); err != nil {
		return err
	}
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

	var add []string
	adds := false
	for _, line := range strings.Split(strings.TrimRight(string(fragment), "\n"), "\n") {
		key, ok := assignment(line)
		if ok && present[key] {
			continue
		}
		adds = adds || ok
		add = append(add, line)
	}
	if !adds {
		return nil
	}

	body := string(current)
	if body != "" && !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	body += "\n" + strings.Join(add, "\n") + "\n"
	return os.WriteFile(file, []byte(body), 0644)
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
	return nil
}

func findModuleRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current working directory: %w", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(cwd, "go.mod")); err == nil {
			return cwd, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("failed to stat go.mod: %w", err)
		}
		if cwd == "/" {
			return "", fmt.Errorf("failed to find go.mod")
		}
		cwd = filepath.Dir(cwd)
	}
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
