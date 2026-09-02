package scribus

import (
	scribusModel "{{ .Project }}/domain/{{ .Domain }}/cmd/scribus/model"

	"gitlab.com/gorib/amqp"
	"gitlab.com/gorib/pry"
	"gitlab.com/gorib/whisk/scan"
)

type {{ title .Domain }}Management interface {
	// Methods consumed by this handler go here.
}

func New{{ title .Event }}Command(
	{{ .Domain }}Management {{ title .Domain }}Management,
	logger pry.Logger,
) *{{ .Event }}Command {
	return &{{ .Event }}Command{
		{{ .Domain }}Management: {{ .Domain }}Management,
		logger:                  logger,
	}
}

type {{ .Event }}Command struct {
	{{ .Domain }}Management {{ title .Domain }}Management
	logger                   pry.Logger
}

func (c *{{ .Event }}Command) Keys() []string {
	return []string{"{{ .Key }}"}
}

func (c *{{ .Event }}Command) Run(src amqp.Source) error {
	event, err := scan.Into[scribusModel.{{ title .Event }}](src)
	if err != nil {
		c.logger.Error("Failed to parse event", pry.Err(err), pry.Ctx(src.Context()))
		return nil
	}
	_ = event
	// TODO: implement
	return nil
}
