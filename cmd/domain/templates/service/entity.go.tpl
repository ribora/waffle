package service

import (
	"context"

	"gitlab.com/gorib/criteria"

	"{{ .Project }}/internal/domain/{{ .Domain }}/model"
)

type {{ title .Domain }}Repository interface {
	Get(ctx context.Context, {{ .Domain }}Id int) *model.{{ title .Domain }}
	Save(ctx context.Context, {{ .Domain }} *model.{{ title .Domain }}) *model.{{ title .Domain }}
	List(ctx context.Context, expr criteria.Expression, sort []string, page int, count int) []*model.{{ title .Domain }}
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

func (m *{{ .Domain }}Management) Get{{ title .Domain }}(ctx context.Context, {{ .Domain }}Id int) *model.{{ title .Domain }} {
	return m.{{ .Domain }}Repository.Get(ctx, {{ .Domain }}Id)
}

func (m *{{ .Domain }}Management) List(ctx context.Context, expr criteria.Expression) []*model.{{ title .Domain }} {
	return m.{{ .Domain }}Repository.List(ctx, expr, nil, 0, 0)
}
