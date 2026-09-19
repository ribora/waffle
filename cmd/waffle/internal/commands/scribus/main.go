package scribus

import (
	"context"
	"embed"
	"fmt"
	"os/exec"
	"path/filepath"
	"slices"

	"gitlab.com/gorib/waffle/cmd/waffle/internal/commands/converter"
	"gitlab.com/gorib/waffle/cmd/waffle/internal/commands/whisk"
	"gitlab.com/gorib/waffle/cmd/waffle/internal/pkg/command"
)

//go:embed templates/*
var templates embed.FS

func New() *cmd {
	return &cmd{command.New("make:domain:scribus", "Make scribus handler for the specified domain (use --batch for BatchHandler)", templates, nil).
		WithUsage("--driver <id> [--batch] <domain> <event> [subject]\n" +
			"  driver is the broker module under config/tools/scribus, e.g. --driver amqp\n" +
			"  subject defaults to <event>; give it when the driver's is dotted, e.g. order_paid order.paid")}
}

type cmd struct {
	*command.Command
}

func (c *cmd) Run(ctx context.Context, args ...string) error {
	batch := false
	if i := slices.Index(args, "--batch"); i >= 0 {
		batch = true
		args = append(args[:i], args[i+1:]...)
	}

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
	if err := c.RequireModules(); err != nil {
		return err
	}
	if err := command.RequireDriver(driverPrefix, driver); err != nil {
		return err
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

	projectName, err := c.FindProjectName()
	if err != nil {
		return err
	}

	cfg, err := c.ProjectConfig(projectName)
	if err != nil {
		return err
	}

	params := map[string]string{
		"Domain":  domain,
		"Event":   event,
		"Subject": subject,
		"Project": projectName,
	}

	mode := "single"
	includeTool := "gitlab.com/gorib/waffle/config/tools/scribus/single"
	if batch {
		mode = "batch"
		includeTool = "gitlab.com/gorib/waffle/config/tools/scribus/batch"
	}

	path := filepath.Join("domain", domain)
	sources := map[string]string{
		filepath.Join("templates", mode, "handler.go.tpl"):  filepath.Join(path, "cmd", "scribus", event+".go"),
		filepath.Join("templates", "model", "event.go.tpl"): filepath.Join(path, "cmd", "scribus", "model", event+".go"),
		filepath.Join("templates", mode, "di.go.tpl"):       filepath.Join("config", "di", "di_"+domain+"_"+event+"_scribus.go"),
	}

	if err := c.Write(sources, params); err != nil {
		return err
	}
	if cfg.Mapper == "goverter" {
		if err := converter.Generate(filepath.Join(path, "cmd", "scribus", "convert"), projectName); err != nil {
			return err
		}
	}

	if err := c.UncommentModule(driverPrefix + driver); err != nil {
		return err
	}
	if err := c.UncommentModule(includeTool); err != nil {
		return err
	}

	if err := whisk.Ensure(c.Command); err != nil {
		return err
	}
	info, err := exec.Command("make", ".tidy", "generate").CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %v", err, string(info))
	}
	return nil
}
