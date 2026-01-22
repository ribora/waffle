package postgres

import (
	"context"
	"errors"

	"github.com/samber/lo"

	"gitlab.com/gorib/criteria"
	"gitlab.com/gorib/pry"
	"gitlab.com/gorib/storage/sql"

	"{{ .Project }}/domain/{{ .Domain }}/model"
)

var (
	{{ .Domain }}Table = sql.Table("{{ plural .Domain }}")
)

func New{{ title .Domain }}Repository(
	db sql.Db,
	logger pry.Logger,
) (*{{ .Domain }}Repository, error) {
	repo, err := sql.NewRepository[stored{{ title .Domain }}](db)
	if err != nil {
		return nil, err
	}
	return &{{ .Domain }}Repository{
		repo: repo,
		logger: logger,
	}, nil
}

type {{ .Domain }}Repository struct {
	repo sql.Repository[stored{{ title .Domain }}]
	logger pry.Logger
}

func (r *{{ .Domain }}Repository) Get(ctx context.Context, {{ .Domain }}Id int) (*model.{{ title .Domain }}, error) {
	stored, err := r.repo.Get(ctx, sql.Select().From({{ .Domain }}Table).Where(criteria.And("id", "eq", {{ .Domain }}Id)))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, model.Err{{ title .Domain }}NotFound
		}
		r.logger.Error(err, pry.Ctx(ctx))
		return nil, sql.ErrQueryFailed
	}
	return stored.Unwrap(), nil
}

func (r *{{ .Domain }}Repository) Save(ctx context.Context, {{ .Domain }} *model.{{ title .Domain }}) (_ *model.{{ title .Domain }}, err error) {
	stored := newStored{{ title .Domain }}({{ .Domain }})
	if {{ .Domain }}.Id != 0 {
		stored, err = r.repo.Get(ctx, sql.Update(stored.Updates()).Table({{ .Domain }}Table).Where(criteria.And("id", "eq", {{ .Domain }}.Id)))
	} else {
		fields, values := stored.Inserts()
		stored, err = r.repo.Get(ctx, sql.Insert(fields...).Into({{ .Domain }}Table).Values(values...))
	}
	if err != nil {
		r.logger.Error(err, pry.Ctx(ctx))
		return nil, sql.ErrQueryFailed
	}
	return stored.Unwrap(), nil
}

func (r *{{ .Domain }}Repository) List(ctx context.Context, expr criteria.Expression, sort []string, page int, count int) ([]*model.{{ title .Domain }}, error) {
	stored, err := r.repo.Select(ctx, sql.Select().From({{ .Domain }}Table).Where(expr).Sort(sort...).Page(page, count))
	if err != nil {
		r.logger.Error(err, pry.Ctx(ctx))
		return nil, sql.ErrQueryFailed
	}
	return lo.Map(stored, func(stored *stored{{ title .Domain }}, _ int) *model.{{ title .Domain }} {
		return stored.Unwrap()
	}), nil
}

func (r *{{ .Domain }}Repository) Count(ctx context.Context, query criteria.Expression) (int, error) {
	return r.repo.Count(ctx, sql.Count().From({{ .Domain }}Table).Where(query))
}
