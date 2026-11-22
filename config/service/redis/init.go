package redis

import (
	"github.com/ribora/diogen/di"
	"github.com/ribora/env"
	"github.com/ribora/redis"
)

func InitRedis() {
	di.Wire[redis.Db](redis.NewDb, di.Args(map[string]any{
		"host":     env.NeedValue[string]("REDIS_HOST"),
		"port":     env.Value("REDIS_PORT", "6379"),
		"password": env.Value("REDIS_PASSWORD", ""),
		"dbIndex":  env.Value("REDIS_DB", 0),
	}), di.Fallback())
}
