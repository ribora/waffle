package command

import (
	"bytes"
	"embed"
	"fmt"
	"path/filepath"
	"text/template"

	"gitlab.com/gorib/waffle/cmd/waffle/internal/pkg/plural"
)

type Templates struct {
	files embed.FS
	funcs template.FuncMap
}

func NewTemplates(files embed.FS) Templates {
	return Templates{files: files, funcs: Funcs()}
}

// Exported so a test renders with the scaffolder's functions, not a copy.
func Funcs() template.FuncMap {
	return template.FuncMap{
		"plural": plural.Plural,
		"title":  Title,
		"camel":  Camel,
	}
}

func (t Templates) Render(source string, params any) ([]byte, error) {
	tpl, err := template.New(filepath.Base(source)).Funcs(t.funcs).ParseFS(t.files, source)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template %s: %w", source, err)
	}
	var out bytes.Buffer
	if err = tpl.Execute(&out, params); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
