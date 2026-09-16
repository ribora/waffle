package service

import (
{{- if .WithRepository }}
	"context"

	"gitlab.com/gorib/criteria"
{{- end }}

	"{{ .Project }}/domain/{{ .Domain }}/model"
)
{{ if .WithRepository }}
func New{{ title .Domain }}Management(
	{{ camel .Domain }}Repository {{ title .Domain }}Repository,
) *{{ camel .Domain }}Management {
	return &{{ camel .Domain }}Management{
		{{ camel .Domain }}Repository: {{ camel .Domain }}Repository,
	}
}

type {{ camel .Domain }}Management struct {
	{{ camel .Domain }}Repository {{ title .Domain }}Repository
}

func (m *{{ camel .Domain }}Management) Get{{ title .Domain }}(ctx context.Context, {{ camel .Domain }}Id int) (*model.{{ title .Domain }}, error) {
	return m.{{ camel .Domain }}Repository.Get(ctx, {{ camel .Domain }}Id)
}

func (m *{{ camel .Domain }}Management) List(ctx context.Context, expr criteria.Expression) ([]*model.{{ title .Domain }}, error) {
	return m.{{ camel .Domain }}Repository.List(ctx, expr, nil, 0, 0)
}

func (m *{{ camel .Domain }}Management) ListPaged(ctx context.Context, expr criteria.Expression, sort []string, page, count int) ([]*model.{{ title .Domain }}, int, error) {
	items, err := m.{{ camel .Domain }}Repository.List(ctx, expr, sort, page, count)
	if err != nil {
		return nil, 0, err
	}
	total, err := m.{{ camel .Domain }}Repository.Count(ctx, expr)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}
{{- else }}
var _ = model.{{ title .Domain }}{}

func New{{ title .Domain }}Management() *{{ camel .Domain }}Management {
	return &{{ camel .Domain }}Management{}
}

type {{ camel .Domain }}Management struct{}
{{- end }}
