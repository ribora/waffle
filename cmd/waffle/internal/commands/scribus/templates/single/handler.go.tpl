package scribus

import (
	"context"

	scribusModel "{{ .Project }}/domain/{{ .Domain }}/cmd/scribus/model"
	"{{ .Project }}/pkg/validator"

	"gitlab.com/gorib/pry"
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
	return []string{"{{ .Event }}"}
}

func (c *{{ .Event }}Command) Run(ctx context.Context, payload []byte) error {
	event, err := validator.Parse[scribusModel.{{ title .Event }}](payload)
	if err != nil {
		c.logger.Error("Failed to parse event", pry.Err(err), pry.Ctx(ctx))
		return nil
	}
	_ = event
	// TODO: implement
	return nil
}
