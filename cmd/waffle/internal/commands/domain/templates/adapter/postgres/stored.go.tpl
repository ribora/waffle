//go:generate go run gitlab.com/gorib/storage/sql/cmd/store $GOFILE

package postgres

import (
	"time"

	"{{ .Project }}/domain/{{ .Domain }}/model"
)

type stored{{ title .Domain }} struct {
    _ *model.{{ title .Domain }}
	Id        int        `db:"id" store:",readonly"`
	CreatedAt time.Time  `db:"created_at" store:",readonly"`
	UpdatedAt *time.Time `db:"updated_at" store:",readonly"`
}
