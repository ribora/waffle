package repository

import (
	"context"
	db "database/sql"
	"errors"

	"gitlab.trgdev.com/gotrg/white-label/modules/gotlib/lib/list"
	"gitlab.trgdev.com/gotrg/white-label/modules/gotlib/lib/match"

	"gitlab.com/gorib/criteria"
	"gitlab.com/gorib/storage"
	"gitlab.com/gorib/storage/sql"
	"gitlab.com/gorib/try"

	"{{ .Project }}/internal/domain/{{ .Domain }}/model"
)

const (
	{{ .Domain }}Table = "{{ plural .Domain }}"
)

func New{{ title .Domain }}Repository(db sql.Db) *{{ .Domain }}Repository {
	return &{{ .Domain }}Repository{repo: sql.NewRepository[stored{{ title .Domain }}](db)}
}

type {{ .Domain }}Repository struct {
	repo storage.SqlRepository[stored{{ title .Domain }}]
}

func (r *{{ .Domain }}Repository) Get(ctx context.Context, {{ .Domain }}Id int) ({{ .Domain }} *model.{{ title .Domain }}, error) {
	stored, err := r.repo.Get(ctx, sql.NewBuilder({{ .Domain }}Table).Where(criteria.And("id", "eq", {{ .Domain }}Id)))
	if err != nil {
		return nil, match.If(errors.Is(err, db.ErrNoRows), model.Err{{ title .Domain }}NotFound, err)
	}
	return stored.Unwrap(), nil
}

func (r *{{ .Domain }}Repository) Save(ctx context.Context, {{ .Domain }} *model.{{ title .Domain }}) (*model.{{ title .Domain }}, error) {
	var query sql.Builder
	if {{ .Domain }}.Id != 0 {
		query = sql.NewBuilder({{ .Domain }}Table).Update(newStored{{ title .Domain }}({{ .Domain }}).Updates()).Where(criteria.And("id", "eq", {{ .Domain }}.Id))
	} else {
		fields, values := newStored{{ title .Domain }}({{ .Domain }}).Inserts()
		query = sql.NewBuilder({{ .Domain }}Table).Insert(fields...).Values(values...)
	}
	stored, err := r.repo.Get(ctx, query)
	if err != nil {
		return nil, err
	}
	return stored.Unwrap(), nil
}

func (r *{{ .Domain }}Repository) List(ctx context.Context, expr criteria.Expression, sort []string, page int, count int) ([]*model.{{ title .Domain }}, error) {
	stored, err := r.repo.Select(ctx, sql.NewBuilder({{ .Domain }}Table).Where(expr).Sort(sort...).Page(page, count))
	if err != nil {
		return nil, err
	}
	return list.Map(stored, func(stored *stored{{ title .Domain }}) *model.{{ title .Domain }} {
		return stored.Unwrap()
	})
}

func (r *{{ .Domain }}Repository) Count(ctx context.Context, query criteria.Expression) int, error {
	return r.SqlRepository.Count(ctx, sql.NewBuilder({{ .Domain }}Table).Where(query))
}
