package scribus

import (
	scribusModel "{{ .Project }}/domain/{{ .Domain }}/cmd/scribus/model"

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
	return []string{"{{ .Subject }}"}
}

func (c *{{ camel .Event }}Command) Run(msgs []scan.Source) error {
	var (
		skipped int
		cause   error
	)
	for _, m := range msgs {
		event, err := scan.Into[scribusModel.{{ title .Event }}](m)
		if err != nil {
			skipped++
			if cause == nil {
				cause = err
			}
			continue
		}
		_ = event
		// TODO: implement
	}
	if skipped > 0 {
		c.logger.Error("Skipped unparsable events",
			pry.Err(cause), pry.Field("count", skipped), pry.Ctx(msgs[len(msgs)-1].Context()))
	}
	return nil
}
