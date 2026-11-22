package scribus

import (
	scribusModel "{{ .Project }}/domain/{{ .Domain }}/cmd/scribus/model"

	"github.com/ribora/pry"
	"github.com/ribora/whisk/scan"
)

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
	return []string{"{{ .Subject }}"}
}

func (c *{{ camel .Event }}Command) Run(src scan.Source) error {
	event, err := scan.Into[scribusModel.{{ title .Event }}](src)
	if err != nil {
		c.logger.Error("Failed to parse event", pry.Err(err), pry.Ctx(src.Context()))
		return nil
	}
	_ = event
	// TODO: implement
	return nil
}
