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

func (c *{{ .Event }}Command) Run(msgs []amqp.Source) error {
	for _, m := range msgs {
		event, err := scan.Into[scribusModel.{{ title .Event }}](m)
		if err != nil {
			c.logger.Error("Failed to parse event", pry.Err(err), pry.Ctx(m.Context()), pry.Field("key", m.RoutingKey()))
			continue
		}
		_ = event
		// TODO: implement
	}
	return nil
}
