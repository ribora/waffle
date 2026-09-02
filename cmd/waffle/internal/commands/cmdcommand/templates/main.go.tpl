package {{ .Command }}

import (
	"context"
	"fmt"
	"os"

	"github.com/pborman/getopt/v2"

	"gitlab.com/gorib/waffle/app"
)

type {{ title .Domain }}Management interface {
	// Methods consumed by this command go here.
}

func New{{ title .Command }}Command(
	{{ .Domain }}Management {{ title .Domain }}Management,
) *{{ .Command }}Command {
	return &{{ .Command }}Command{
		BaseCommand:             app.NewCommand("{{ .Command }}", "{{ title .Command }} {{ .Domain }}"),
		{{ .Domain }}Management: {{ .Domain }}Management,
	}
}

type {{ .Command }}Command struct {
	app.BaseCommand
	{{ .Domain }}Management {{ title .Domain }}Management
}

func (c *{{ .Command }}Command) Run(ctx context.Context) error {
	if err := c.Usage(ctx); err != nil {
		return err
	}
	// TODO: implement
	return nil
}

// Own option set, and argv from the context rather than os.Args: cron runs
// commands inside this same binary, and the global flag set is shared.
func (c *{{ .Command }}Command) Usage(ctx context.Context) error {
	opts := getopt.New()
	var help bool
	opts.FlagLong(&help, "help", 'h', "Display this help message")
	if err := opts.Getopt(app.Args(ctx), nil); err != nil {
		return err
	}
	if help {
		opts.SetParameters("")
		_, _ = fmt.Fprintf(os.Stderr, "%s\n", c.Description())
		opts.PrintUsage(os.Stderr)
		return app.ErrUsage
	}
	return nil
}
