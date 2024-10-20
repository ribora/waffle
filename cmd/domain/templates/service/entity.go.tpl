package service

import (
	"context"

	"gitlab.com/gorib/criteria"

	"{{ .Project }}/internal/domain/{{ .Domain }}/model"
)

type {{ .TitleDomain }}Repository interface {
	Get(ctx context.Context, {{ .Domain }}Id int) *model.{{ .TitleDomain }}
	Save(ctx context.Context, {{ .Domain }} *model.{{ .TitleDomain }}) *model.{{ .TitleDomain }}
	List(ctx context.Context, expr criteria.Expression, sort []string, page int, count int) []*model.{{ .TitleDomain }}
}

func New{{ .TitleDomain }}Management(
	{{ .Domain }}Repository {{ .TitleDomain }}Repository,
) *{{ .Domain }}Management {
	return &{{ .Domain }}Management{
		{{ .Domain }}Repository: {{ .Domain }}Repository,
	}
}

type {{ .Domain }}Management struct {
	{{ .Domain }}Repository {{ .TitleDomain }}Repository
}

func (m *{{ .Domain }}Management) Get{{ .TitleDomain }}(ctx context.Context, {{ .Domain }}Id int) *model.{{ .TitleDomain }} {
	return m.{{ .Domain }}Repository.Get(ctx, {{ .Domain }}Id)
}

func (m *{{ .Domain }}Management) List(ctx context.Context, expr criteria.Expression) []*model.{{ .TitleDomain }} {
	return m.{{ .Domain }}Repository.List(ctx, expr, nil, 0, 0)
}
