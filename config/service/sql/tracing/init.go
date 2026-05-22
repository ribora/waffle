package tracing

import (
	"gitlab.com/gorib/di"
	"gitlab.com/gorib/env"
	"gitlab.com/gorib/storage/sql"
)

func InitTracingDb() {
	di.Wire[sql.Db](sql.NewTracedDb, di.Args(map[string]any{
		"driver": env.NeedValue[string]("DB_DRIVER"),
		"dsn":    env.NeedValue[string]("DB_DSN"),
	}))
}
