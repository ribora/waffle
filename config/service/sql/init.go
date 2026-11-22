package sql

import (
	"github.com/ribora/diogen/di"
	"github.com/ribora/env"
	"github.com/ribora/sql"
)

func InitDb() {
	di.Wire[sql.Db](sql.NewDb, di.Args(map[string]any{
		"driver": env.NeedValue[string]("DB_DRIVER"),
		"dsn":    env.NeedValue[string]("DB_DSN"),
	}), di.Fallback())
}
