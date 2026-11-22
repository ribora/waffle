package postgres

import (
	"context"
	"errors"

	"github.com/samber/lo"

	"github.com/ribora/criteria"
	"github.com/ribora/failure"
	"github.com/ribora/pry"
	"github.com/ribora/sql"

	"{{ .Project }}/domain/{{ .Domain }}/model"
)

var (
	{{ camel .Entity }}Table = sql.Table("{{ plural .Entity }}")
)

func New{{ title .Entity }}Repository(
	db sql.Db,
	logger pry.Logger,
) (*{{ camel .Entity }}Repository, error) {
	repo, err := sql.NewRepository[stored{{ title .Entity }}](db)
	if err != nil {
		return nil, err
	}
	return &{{ camel .Entity }}Repository{
		repo:   repo,
		logger: logger,
	}, nil
}

type {{ camel .Entity }}Repository struct {
	repo   sql.Repository[stored{{ title .Entity }}]
	logger pry.Logger
}

func (r *{{ camel .Entity }}Repository) Get(ctx context.Context, {{ camel .Entity }}Id int) (*model.{{ title .Entity }}, error) {
	stored, err := r.repo.Get(ctx, sql.Select().From({{ camel .Entity }}Table).Where(criteria.And("id", "eq", {{ camel .Entity }}Id)))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, failure.Parse(model.Err{{ title .Entity }}NotFound)
		}
		if ctx.Err() == nil {
			r.logger.Error(err, pry.Ctx(ctx))
		}
		return nil, failure.Parse(sql.ErrQueryFailed)
	}
	return stored.Unwrap(), nil
}

func (r *{{ camel .Entity }}Repository) Save(ctx context.Context, {{ camel .Entity }} *model.{{ title .Entity }}) (_ *model.{{ title .Entity }}, err error) {
	stored := newStored{{ title .Entity }}({{ camel .Entity }})
	if {{ camel .Entity }}.Id != 0 {
		stored, err = r.repo.Get(ctx, sql.Update(stored.Updates()).Table({{ camel .Entity }}Table).Where(criteria.And("id", "eq", {{ camel .Entity }}.Id)))
	} else {
		fields, values := stored.Inserts()
		stored, err = r.repo.Get(ctx, sql.Insert(fields...).Into({{ camel .Entity }}Table).Values(values...))
	}
	if err != nil {
		if ctx.Err() == nil {
			r.logger.Error(err, pry.Ctx(ctx))
		}
		return nil, failure.Parse(sql.ErrQueryFailed)
	}
	return stored.Unwrap(), nil
}

func (r *{{ camel .Entity }}Repository) List(ctx context.Context, expr criteria.Expression, sort []string, page int, count int) ([]*model.{{ title .Entity }}, error) {
	stored, err := r.repo.Select(ctx, sql.Select().From({{ camel .Entity }}Table).Where(expr).Sort(sort...).Page(page, count))
	if err != nil {
		if ctx.Err() == nil {
			r.logger.Error(err, pry.Ctx(ctx))
		}
		return nil, failure.Parse(sql.ErrQueryFailed)
	}
	return lo.Map(stored, func(stored *stored{{ title .Entity }}, _ int) *model.{{ title .Entity }} {
		return stored.Unwrap()
	}), nil
}

func (r *{{ camel .Entity }}Repository) Count(ctx context.Context, query criteria.Expression) (int, error) {
	return r.repo.Count(ctx, sql.Count().From({{ camel .Entity }}Table).Where(query))
}
