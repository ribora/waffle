package service

import (
	"context"

	"gitlab.com/gorib/criteria"

	"{{ .Project }}/domain/{{ .Domain }}/model"
)

type {{ title .Entity }}Repository interface {
	Get(ctx context.Context, {{ .Entity }}Id int) (*model.{{ title .Entity }}, error)
	Save(ctx context.Context, {{ .Entity }} *model.{{ title .Entity }}) (*model.{{ title .Entity }}, error)
	List(ctx context.Context, query criteria.Expression, sort []string, page int, count int) ([]*model.{{ title .Entity }}, error)
	Count(ctx context.Context, query criteria.Expression) (int, error)
}
