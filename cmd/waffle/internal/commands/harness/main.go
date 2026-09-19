package harness

import (
	"context"
	"embed"
	"fmt"
	"go/format"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"gitlab.com/gorib/waffle/cmd/waffle/internal/pkg/command"
	"gitlab.com/gorib/waffle/replacements"
)

//go:embed templates/*
var templates embed.FS

func New() *cmd {
	return &cmd{command.New("tests:harness",
		"Report the component harness, and write what is missing with init",
		templates, nil).
		WithUsage("[init]\n" +
			"  init  write the files that are not there yet, leaving the rest alone")}
}

type cmd struct {
	*command.Command
}

func (c *cmd) Run(_ context.Context, args ...string) error {
	if err := c.Operands(args, 0, 1); err != nil {
		return err
	}
	write := len(args) == 1
	if write && args[0] != "init" {
		return fmt.Errorf("unknown subcommand %q\n%s", args[0], c.Usage())
	}

	sources, params, err := c.plan()
	if err != nil {
		return err
	}
	if !write {
		return state(sources)
	}
	return c.scaffold(sources, params)
}

func (c *cmd) scaffold(sources map[string]string, params map[string]any) error {
	fresh := absent(sources)
	if err := c.Write(sources, params); err != nil {
		return err
	}
	// Conditional imports come out as Go but not as gofmt's Go. Only what this
	// run wrote: reformatting somebody's edited file is a change nobody asked for.
	for _, target := range fresh {
		if err := reformat(target); err != nil {
			return err
		}
	}
	return nil
}

func state(sources map[string]string) error {
	targets := slices.Sorted(maps(sources))
	var present, missing []string
	for _, target := range targets {
		if _, err := os.Stat(target); err == nil {
			present = append(present, target)
			continue
		}
		missing = append(missing, target)
	}
	command.ReportKept(present...)
	command.ReportMissing(missing...)
	if len(missing) > 0 {
		command.ReportWarning("run `waffle tests:harness init` to write them")
	}
	return nil
}

func absent(sources map[string]string) []string {
	var out []string
	for _, target := range sources {
		if _, err := os.Stat(target); err != nil {
			out = append(out, target)
		}
	}
	return out
}

func reformat(target string) error {
	if !strings.HasSuffix(target, ".go") {
		return nil
	}
	body, err := os.ReadFile(target)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", target, err)
	}
	formatted, err := format.Source(body)
	if err != nil {
		return fmt.Errorf("the scaffolded %s does not parse: %w", target, err)
	}
	return os.WriteFile(target, formatted, 0o644)
}

func maps(sources map[string]string) func(func(string) bool) {
	return func(yield func(string) bool) {
		for _, target := range sources {
			if !yield(target) {
				return
			}
		}
	}
}

func (c *cmd) plan() (map[string]string, map[string]any, error) {
	module, err := c.FindProjectName()
	if err != nil {
		return nil, nil, err
	}
	containers, err := replacements.DiscoverContainers(".")
	if err != nil {
		return nil, nil, err
	}
	container := containers[0]
	importPath := path.Join(module, filepath.ToSlash(container.Dir))

	seams, err := replacements.ReadSeams(container, importPath)
	if err != nil {
		return nil, nil, err
	}
	found, err := read(container, seams)
	if err != nil {
		return nil, nil, err
	}

	service := path.Base(module)
	p := map[string]any{
		"Module":           module,
		"Service":          service,
		"Env":              strings.ToUpper(strings.ReplaceAll(service, "-", "_")),
		"Container":        importPath,
		"Package":          container.Package,
		"Postgres":         found.Postgres,
		"Redis":            found.Redis,
		"Resources":        registry(found),
		"Logger":           found.Logger,
		"Hilton":           found.Hilton,
		"Consumer":         found.Consumer,
		"Migrations":       migrations(module),
		"Dead":             dead(found.Dead),
		"GrpcHandlers":     found.Grpc.Handlers,
		"GrpcInterceptors": found.Grpc.Interceptors,
		"GrpcStream":       found.Grpc.Stream,
		"HttpMux":          found.Http.Mux,
		"HttpGlobal":       found.Http.Global,
		"HttpRouters":      found.Http.Routers,
	}

	// tests/feature holds the .feature files alone: only one of the two is a
	// specification, and it is read by someone who does not read the other.
	harnessDir := filepath.Join("tests", "internal", "harness")
	testsDir := "tests"
	sources := map[string]string{
		tpl("resources.go.tpl"):         filepath.Join(harnessDir, "resources.go"),
		tpl("boot.go.tpl"):              filepath.Join(harnessDir, "boot.go"),
		tpl("suite_test.go.tpl"):        filepath.Join(testsDir, "suite_test.go"),
		tpl("boot_test.go.tpl"):         filepath.Join(testsDir, "boot_test.go"),
		tpl("replacements_test.go.tpl"): filepath.Join(container.Dir, "replacements_test.go"),
		tpl("boundaries_test.go.tpl"):   filepath.Join(container.Dir, "boundaries_test.go"),
	}
	// Both when the service serves both — two of nine in sherpa do, and two files
	// could not coexist: each would declare Clients and Serve in one package.
	if found.Grpc.Present() || found.Http.Present() {
		sources[tpl("serve.go.tpl")] = filepath.Join(harnessDir, "serve.go")
	}
	return sources, p, nil
}

func tpl(name string) string { return filepath.Join("templates", name) }

// The literal resource.New is given, in the order the systems are declared
// above it. Empty for a service that reaches no infrastructure at all, and Up,
// Reset and Down then do nothing.
func registry(found shape) []string {
	var out []string
	if found.Postgres {
		out = append(out, "postgres")
	}
	if found.Redis {
		out = append(out, "cache")
	}
	return out
}

func dead(conns []conn) []map[string]string {
	out := make([]map[string]string, 0, len(conns))
	for _, c := range conns {
		out = append(out, map[string]string{"Field": c.Field, "Name": c.Name})
	}
	return out
}

// The migration packages a domain gets from make:domain:migrate. Discovered
// rather than configured: the directory is where the command puts it.
func migrations(module string) []map[string]string {
	entries, err := os.ReadDir("domain")
	if err != nil {
		return nil
	}
	var out []map[string]string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join("domain", entry.Name(), "cmd", "migrate")
		if _, err := os.Stat(dir); err != nil {
			continue
		}
		out = append(out, map[string]string{
			"Alias": entry.Name() + "Migrate",
			"Path":  path.Join(module, filepath.ToSlash(dir)),
		})
	}
	slices.SortFunc(out, func(a, b map[string]string) int { return strings.Compare(a["Alias"], b["Alias"]) })
	return out
}
