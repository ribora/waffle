package sql

import (
	"gitlab.com/gorib/di"
	"gitlab.com/gorib/env"
	"gitlab.com/gorib/sql"
)

func InitDb() {
	di.Wire[sql.Db](sql.NewDb, di.Args(map[string]any{
		"driver": env.NeedValue[string]("DB_DRIVER"),
		"dsn":    env.NeedValue[string]("DB_DSN"),
	}))
}
