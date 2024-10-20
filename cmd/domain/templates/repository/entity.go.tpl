//go:generate go run gitlab.com/gorib/storage/sql/cmd/store $GOFILE

package repository

import (
	"context"
	db "database/sql"
	"errors"
	"time"

	"gitlab.trgdev.com/gotrg/white-label/modules/gotlib/lib/list"

	"gitlab.com/gorib/criteria"
	"gitlab.com/gorib/storage"
	"gitlab.com/gorib/storage/sql"
	"gitlab.com/gorib/try"

	"{{ .Project }}/internal/domain/{{ .Domain }}/model"
)

const (
	{{ .Domain }}Table = "{{ .PluralsDomain }}"
)

func New{{ .TitleDomain }}Repository(repo storage.Repository) *{{ .Domain }}Repository {
	return &{{ .Domain }}Repository{Repository: repo}
}

// sql.store *model.{{ .TitleDomain }}
type stored{{ .TitleDomain }} struct {
	Id        int        `db:"id" store:",readonly"`
	CreatedAt time.Time  `db:"created_at" store:",readonly"`
	UpdatedAt *time.Time `db:"updated_at" store:",readonly"`
}

type {{ .Domain }}Repository struct {
	storage.Repository
}

func (r *{{ .Domain }}Repository) Get(ctx context.Context, {{ .Domain }}Id int) ({{ .Domain }} *model.{{ .TitleDomain }}) {
	ctx, commit, rollback := r.Begin(ctx)
	defer rollback()
	try.Catch(func() {
		stored := try.Throw(sql.Get[stored{{ .TitleDomain }}](ctx, sql.NewBuilder({{ .Domain }}Table).Where(criteria.And("id", "eq", {{ .Domain }}Id))))
		{{ .Domain }} = stored.Unwrap()
	}, func(throwable error) {
		if errors.Is(throwable, db.ErrNoRows) {
			panic(model.Err{{ .TitleDomain }}NotFound)
		}
		panic(throwable)
	})
	commit()
	return {{ .Domain }}
}

func (r *{{ .Domain }}Repository) Save(ctx context.Context, {{ .Domain }} *model.{{ .TitleDomain }}) *model.{{ .TitleDomain }} {
	ctx, commit, rollback := r.Begin(ctx)
	defer rollback()
	stored := *newStored{{ .TitleDomain }}({{ .Domain }})
	if {{ .Domain }}.Id != 0 {
		stored = try.Throw(sql.Get[stored{{ .TitleDomain }}](ctx, sql.NewBuilder({{ .Domain }}Table).Update(stored.Updates()).Where(criteria.And("id", "eq", {{ .Domain }}.Id))))
	} else {
		fields, values := stored.Inserts()
		stored = try.Throw(sql.Get[stored{{ .TitleDomain }}](ctx, sql.NewBuilder({{ .Domain }}Table).Insert(fields...).Values(values...)))
	}
	commit()
	return stored.Unwrap()
}

func (r *{{ .Domain }}Repository) List(ctx context.Context, expr criteria.Expression, sort []string, page int, count int) []*model.{{ .TitleDomain }} {
	ctx, commit, rollback := r.Begin(ctx)
	defer rollback()
	stored := try.Throw(sql.Select[[]*stored{{ .TitleDomain }}](ctx, sql.NewBuilder({{ .Domain }}Table).Where(expr).Sort(sort...).Page(page, count)))
	commit()
	return list.Map(stored, func(stored *stored{{ .TitleDomain }}) *model.{{ .TitleDomain }} {
		return stored.Unwrap()
	})
}
