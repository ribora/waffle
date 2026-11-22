package outbox

import (
	"context"
	"embed"
	"errors"
	"path/filepath"
	"slices"
	"time"

	"github.com/ribora/waffle/cmd/waffle/internal/commands/migrate"
	"github.com/ribora/waffle/cmd/waffle/internal/pkg/command"
)

//go:embed templates/*
var templateFiles embed.FS

var templates = command.NewTemplates(templateFiles)

const domain = "outbox"

const (
	module       = "github.com/ribora/waffle/config/tools/outbox"
	driverPrefix = module + "/"
)

const migrationSuffix = "_create_outbox_table.sql"

// The driver has no default, for the reason make:domain:scribus has none: picking
// a broker is the question the module was split not to answer.
var errNoDriver = errors.New("--driver is required: it names the broker module under config/tools/outbox, e.g. --driver amqp")

func New() *cmd {
	return &cmd{command.New("make:outbox", "Enable the transactional outbox and write its table").
		WithUsage("--driver <id>\n" +
			"  driver is the broker module under config/tools/outbox, e.g. --driver amqp")}
}

type cmd struct {
	*command.Command
}

func (c *cmd) Run(ctx context.Context, args ...string) error {
	driver, args, err := command.TakeDriver(args)
	if err != nil {
		return err
	}
	if err := c.Operands(args, 0, 0); err != nil {
		return err
	}
	if driver == "" {
		return errNoDriver
	}
	p, err := command.OpenProject()
	if err != nil {
		return err
	}
	// The driver only: it blank-imports the module, so modules.go carries one line.
	if err := p.RequireModules(slices.Concat(command.MigrationModules, []string{driverPrefix + driver})...); err != nil {
		return err
	}

	if err := migrate.Runner(p, domain); err != nil {
		return err
	}
	if err := migration(p); err != nil {
		return err
	}
	if err := p.EnableStorage(); err != nil {
		return err
	}
	if err := p.EnableModule(driverPrefix + driver); err != nil {
		return err
	}
	reportWiring()

	_, err = p.Run("make", ".tidy", "generate")
	return err
}

func migration(p *command.Project) error {
	dir := migrate.Dir(domain)
	found, err := p.Glob(filepath.Join(dir, "*"+migrationSuffix))
	if err != nil {
		return err
	}
	if len(found) > 0 {
		command.ReportKept(found...)
		return nil
	}
	name := time.Now().Format("20060102150405") + migrationSuffix
	return p.Write(templates, map[string]string{
		filepath.Join("templates", "migration.sql.tpl"): filepath.Join(dir, name),
	}, nil)
}

func reportWiring() {
	command.ReportBlock(
		"to publish through the outbox, widen your publisher interface and point it at the module's instance:",
		"// pkg/publisher/publisher.go\n"+
			"type Publisher interface {\n"+
			"\tPublish(ctx context.Context, subject string, document any) error\n"+
			"\tEnqueue(ctx context.Context, subject string, document any) error\n"+
			"}\n"+
			"\n"+
			"// config/di/di.go — replaces the amqp.NewPublisher registration, if you have one\n"+
			"di.Wire[publisher.Publisher](func(p outbox.Publisher) publisher.Publisher { return p })",
	)
	command.ReportBlock(
		"then change the call sites that must not lose their event, one at a time:",
		"-  return m.publisher.Publish(ctx, \"invoices.paid\", event)\n"+
			"+  return m.publisher.Enqueue(ctx, \"invoices.paid\", event)\n"+
			"\n"+
			"Enqueue is atomic with the transaction the context carries, so the call site\n"+
			"has to own one — a repository that commits on its own is not enough.",
	)
	command.ReportBlock(
		"and run the relay as its own deployment — nothing is delivered without it:",
		"./app outbox\n"+
			"./app outbox status   # unsent, failed, and the age of the oldest entry",
	)
}
