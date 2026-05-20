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
	if err := c.Usage(); err != nil {
		return err
	}
	// TODO: implement
	_ = ctx
	return nil
}

func (c *{{ .Command }}Command) Usage() error {
	help := getopt.BoolLong("help", 'h', "Show this message")
	getopt.Parse()

	if help != nil && *help {
		getopt.PrintUsage(os.Stderr)
		os.Exit(2)
	}

	_ = fmt.Sprintf
	return nil
}
