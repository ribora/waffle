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

func (r *{{ .Domain }}Repository) Get(ctx context.Context, {{ .Domain }}Id int) ({{ .Domain }} *model.{{ title .Domain }}) {
	stored, err := r.repo.Get(ctx, sql.NewBuilder({{ .Domain }}Table).Where(criteria.And("id", "eq", {{ .Domain }}Id)))
	try.ThrowError(match.If(errors.Is(err, db.ErrNoRows), model.Err{{ title .Domain }}NotFound, err))
	return stored.Unwrap()
}

func (r *{{ .Domain }}Repository) Save(ctx context.Context, {{ .Domain }} *model.{{ title .Domain }}) *model.{{ title .Domain }} {
	stored := *newStored{{ title .Domain }}({{ .Domain }})
	if {{ .Domain }}.Id != 0 {
		stored = try.Throw(r.repo.Get(ctx, sql.NewBuilder({{ .Domain }}Table).Update(stored.Updates()).Where(criteria.And("id", "eq", {{ .Domain }}.Id))))
	} else {
		fields, values := stored.Inserts()
		stored = try.Throw(r.repo.Get(ctx, sql.NewBuilder({{ .Domain }}Table).Insert(fields...).Values(values...)))
	}
	return stored.Unwrap()
}

func (r *{{ .Domain }}Repository) List(ctx context.Context, expr criteria.Expression, sort []string, page int, count int) []*model.{{ title .Domain }} {
	stored := try.Throw(r.repo.Select(ctx, sql.NewBuilder({{ .Domain }}Table).Where(expr).Sort(sort...).Page(page, count)))
	return list.Map(stored, func(stored *stored{{ title .Domain }}) *model.{{ title .Domain }} {
	return list.Map(stored, func(stored *stored{{ title .Domain }}) *model.{{ title .Domain }} {
		return stored.Unwrap()
	})
}
