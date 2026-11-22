package scribus

import (
	"context"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ribora/waffle/cmd/waffle/internal/commands/converter"
	"github.com/ribora/waffle/cmd/waffle/internal/commands/whisk"
	"github.com/ribora/waffle/cmd/waffle/internal/pkg/command"
)

//go:embed templates/*
var templateFiles embed.FS

var templates = command.NewTemplates(templateFiles)

func New() *cmd {
	return &cmd{command.New("make:domain:scribus", "Make scribus handler for the specified domain (use --batch for BatchHandler)").
		WithUsage("--driver <id> [--batch] <domain> <event> [subject]\n" +
			"  driver is the broker module under config/tools/scribus, e.g. --driver amqp\n" +
			"  subject defaults to <event>; give it when the driver's is dotted, e.g. order_paid order.paid")}
}

type cmd struct {
	*command.Command
}

func (c *cmd) Run(ctx context.Context, args ...string) error {
	batch, args := command.TakeBool(args, "--batch")
	driver, args, err := command.TakeDriver(args)
	if err != nil {
		return err
	}

	if err := c.Operands(args, 2, 3); err != nil {
		return err
	}
	if driver == "" {
		return errNoDriver
	}
	if err := command.ValidateName("domain", args[0]); err != nil {
		return err
	}
	domain := args[0]
	event := args[1]

	// Its own operand because subjects are dotted and the event has to stay an
	// identifier; the fallback only works when one value fits both jobs.
	subject := event
	if len(args) > 2 {
		subject = args[2]
	}
	if err := command.ValidateName("event", event); err != nil {
		if len(args) > 2 {
			return fmt.Errorf("%w — it names the generated type, the subject is the operand after it", err)
		}
		return fmt.Errorf("%w — it names the generated type and, with no subject given, doubles as the subject.\n"+
			"Give both: waffle %s %s <event> %s", err, c.Code(), domain, event)
	}
	if err := command.ValidateSubject(subject); err != nil {
		return err
	}

	p, err := command.OpenProject()
	if err != nil {
		return err
	}
	if err := p.RequireDomain(domain); err != nil {
		return err
	}
	mode := "single"
	if batch {
		mode = "batch"
	}
	if err := p.RequireModules(driverPrefix+driver, driverPrefix+mode); err != nil {
		return err
	}
	wiring := filepath.Join("config", "di", "di_"+domain+"_"+event+"_scribus.go")
	if err := requireOneConsumer(p, driver, mode, wiring); err != nil {
		return err
	}
	cfg, err := p.Config()
	if err != nil {
		return err
	}

	params := map[string]string{
		"Domain":  domain,
		"Event":   event,
		"Subject": subject,
		"Project": p.Module(),
	}

	path := filepath.Join("domain", domain)
	sources := map[string]string{
		filepath.Join("templates", mode, "handler.go.tpl"):  filepath.Join(path, "cmd", "scribus", event+".go"),
		filepath.Join("templates", "model", "event.go.tpl"): filepath.Join(path, "cmd", "scribus", "model", event+".go"),
		filepath.Join("templates", mode, "di.go.tpl"):       wiring,
	}
	// Every handler of the domain shares these two; one scaffolded by an older waffle keeps them in its own files.
	management := command.Title(domain) + "Management"
	if len(holding(p, filepath.Join(path, "cmd", "scribus"), "type "+management+" interface")) == 0 {
		sources[filepath.Join("templates", "management.go.tpl")] = filepath.Join(path, "cmd", "scribus", "management.go")
	}
	if len(holding(p, filepath.Join("config", "di"), "di.Wire[scribus"+command.Title(domain)+"."+management+"]")) == 0 {
		sources[filepath.Join("templates", "di.go.tpl")] = filepath.Join("config", "di", "di_"+domain+"_scribus.go")
	}

	if err := p.Write(templates, sources, params); err != nil {
		return err
	}
	if cfg.Mapper == "goverter" {
		if err := converter.Generate(p, filepath.Join(path, "cmd", "scribus", "convert")); err != nil {
			return err
		}
	}

	if err := p.EnableModule(driverPrefix + driver); err != nil {
		return err
	}
	if err := p.EnableModule(driverPrefix + mode); err != nil {
		return err
	}

	if err := whisk.Ensure(p); err != nil {
		return err
	}
	_, err = p.Run("make", ".tidy", "generate")
	return err
}

var modes = []string{"single", "batch"}

// Before the first write: each refusal is a binary that would stop at startup.
func requireOneConsumer(p *command.Project, driver, mode, wiring string) error {
	// Both live under driverPrefix, so a mode given as the driver enables two.
	if slices.Contains(modes, driver) {
		return fmt.Errorf("%s is a mode, not a driver — --batch picks the mode", driver)
	}
	active, err := p.ActiveModules()
	if err != nil {
		return err
	}
	for _, module := range active {
		switch name, found := strings.CutPrefix(module, driverPrefix); {
		case !found, name == mode, name == driver:
		case slices.Contains(modes, name):
			return fmt.Errorf("%s enables %s already: single and batch both register the scribus command, so a service takes one mode",
				command.ModulesFile, module)
		default:
			return fmt.Errorf("%s enables %s already: a service consumes through one driver, so give --driver %s",
				command.ModulesFile, module, name)
		}
	}
	if mode != "batch" {
		return nil
	}
	// This event's own file is a rerun, which writes nothing.
	wired := slices.DeleteFunc(holding(p, filepath.Join("config", "di"), ".BatchHandler]("),
		func(file string) bool { return file == wiring })
	if len(wired) > 0 {
		return fmt.Errorf("%s wires a scribus.BatchHandler already, and NewBatchScribusCommand takes exactly one: add the subject to its Keys()",
			wired[0])
	}
	return nil
}

// The files of dir whose text holds code; text, because mid-scaffold the package need not parse.
func holding(p *command.Project, dir, code string) []string {
	files, _ := p.Glob(filepath.Join(dir, "*.go"))
	return slices.DeleteFunc(files, func(file string) bool {
		data, err := os.ReadFile(p.Path(file))
		return err != nil || !strings.Contains(string(data), code)
	})
}
