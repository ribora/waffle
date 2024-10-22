package migrate

import (
	"embed"
)

//go:embed migrations/*
var migrations embed.FS

func NewMigrations() embed.FS {
	return migrations
}
