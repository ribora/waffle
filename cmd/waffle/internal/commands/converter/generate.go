// Package converter scaffolds goverter files: a per-package convert.go plus
// the project-wide whitelist (pkg/goverter/convert.go). Idempotent.
package converter

import (
	"embed"
	"path/filepath"

	"github.com/ribora/waffle/cmd/waffle/internal/pkg/command"
)

//go:embed templates/*
var templateFiles embed.FS

var templates = command.NewTemplates(templateFiles)

// Generate scaffolds the convert package at targetDir, relative to the root.
func Generate(p *command.Project, targetDir string) error {
	params := map[string]string{
		"Package": filepath.Base(targetDir),
		"Project": p.Module(),
	}
	sources := map[string]string{
		filepath.Join("templates", "whitelist.go.tpl"): filepath.Join("pkg", "goverter", "convert.go"),
		filepath.Join("templates", "convert.go.tpl"):   filepath.Join(targetDir, "convert.go"),
	}
	if err := p.Write(templates, sources, params); err != nil {
		return err
	}
	return p.EnsureTool("github.com/ribora/convert/cmd/goverter")
}
