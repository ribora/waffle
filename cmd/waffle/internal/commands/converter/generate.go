// Package converter scaffolds goverter files: a per-package convert.go plus
// the project-wide whitelist (pkg/goverter/convert.go). Idempotent.
package converter

import (
	"embed"
	"path/filepath"

	"gitlab.com/gorib/waffle/cmd/waffle/internal/pkg/command"
)

//go:embed templates/*
var templates embed.FS

// Generate scaffolds the convert package at targetDir. projectName must be the
// full module path — it lands in the generated goverter:extend import.
func Generate(targetDir, projectName string) error {
	c := command.New("converter-internal", "", templates, nil)
	params := map[string]string{
		"Package": filepath.Base(targetDir),
		"Project": projectName,
	}
	sources := map[string]string{
		filepath.Join("templates", "whitelist.go.tpl"): filepath.Join("pkg", "goverter", "convert.go"),
		filepath.Join("templates", "convert.go.tpl"):   filepath.Join(targetDir, "convert.go"),
	}
	if err := c.Write(sources, params); err != nil {
		return err
	}
	return c.EnsureTool("github.com/jmattheis/goverter/cmd/goverter")
}
