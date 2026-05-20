//go:generate go tool store $GOFILE

package postgres

import (
	"time"

	"{{ .Project }}/domain/{{ .Domain }}/model"
)

type stored{{ title .Entity }} struct {
	_ *model.{{ title .Entity }}
	Id        int        `db:"id" store:",readonly"`
	CreatedAt time.Time  `db:"created_at" store:",readonly"`
	UpdatedAt *time.Time `db:"updated_at" store:",readonly"`
}
