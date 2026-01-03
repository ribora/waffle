package migrate

import (
	"embed"
	"io/fs"
)

type Migration interface {
	fs.FS
}

//go:embed migrations/*
var migrations embed.FS

func NewMigrations() embed.FS {
	return migrations
}
