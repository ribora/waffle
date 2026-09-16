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
	{{ camel .Domain }}Management {{ title .Domain }}Management,
	logger pry.Logger,
) *{{ camel .Event }}Command {
	return &{{ camel .Event }}Command{
		{{ camel .Domain }}Management: {{ camel .Domain }}Management,
		logger:                  logger,
	}
}

type {{ camel .Event }}Command struct {
	{{ camel .Domain }}Management {{ title .Domain }}Management
	logger                   pry.Logger
}

func (c *{{ camel .Event }}Command) Keys() []string {
	return []string{"{{ .Key }}"}
}

func (c *{{ camel .Event }}Command) Run(src amqp.Source) error {
	event, err := scan.Into[scribusModel.{{ title .Event }}](src)
	if err != nil {
		c.logger.Error("Failed to parse event", pry.Err(err), pry.Ctx(src.Context()))
		return nil
	}
	_ = event
	// TODO: implement
	return nil
}
