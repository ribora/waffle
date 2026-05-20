package service

import (
{{- if .WithStorage }}
	"context"

	"gitlab.com/gorib/criteria"
{{- end }}

	"{{ .Project }}/domain/{{ .Domain }}/model"
)
{{ if .WithStorage }}
type {{ title .Domain }}Repository interface {
	Get(ctx context.Context, {{ .Domain }}Id int) (*model.{{ title .Domain }}, error)
	Save(ctx context.Context, {{ .Domain }} *model.{{ title .Domain }}) (*model.{{ title .Domain }}, error)
	List(ctx context.Context, expr criteria.Expression, sort []string, page int, count int) ([]*model.{{ title .Domain }}, error)
	Count(ctx context.Context, query criteria.Expression) (int, error)
}

func New{{ title .Domain }}Management(
	{{ .Domain }}Repository {{ title .Domain }}Repository,
) *{{ .Domain }}Management {
	return &{{ .Domain }}Management{
		{{ .Domain }}Repository: {{ .Domain }}Repository,
	}
}

type {{ .Domain }}Management struct {
	{{ .Domain }}Repository {{ title .Domain }}Repository
}

func (m *{{ .Domain }}Management) Get{{ title .Domain }}(ctx context.Context, {{ .Domain }}Id int) (*model.{{ title .Domain }}, error) {
	return m.{{ .Domain }}Repository.Get(ctx, {{ .Domain }}Id)
}

func (m *{{ .Domain }}Management) List(ctx context.Context, expr criteria.Expression) ([]*model.{{ title .Domain }}, error) {
	return m.{{ .Domain }}Repository.List(ctx, expr, nil, 0, 0)
}

func (m *{{ .Domain }}Management) ListPaged(ctx context.Context, expr criteria.Expression, sort []string, page, count int) ([]*model.{{ title .Domain }}, int, error) {
	items, err := m.{{ .Domain }}Repository.List(ctx, expr, sort, page, count)
	if err != nil {
		return nil, 0, err
	}
	total, err := m.{{ .Domain }}Repository.Count(ctx, expr)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}
{{- else }}
var _ = model.{{ title .Domain }}{}

func New{{ title .Domain }}Management() *{{ .Domain }}Management {
	return &{{ .Domain }}Management{}
}

type {{ .Domain }}Management struct{}
{{- end }}
