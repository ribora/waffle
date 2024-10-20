package main

import (
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/dustin/go-humanize/english"
	"golang.org/x/mod/modfile"
)

//go:embed templates/*
var tpls embed.FS

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: domain <domain>")
		os.Exit(1)
	}
	domain := os.Args[1]
	projectName, err := findProjectName()
	if err != nil {
		fmt.Printf("Error finding project name: %v\n", err)
		os.Exit(1)
	}
	err = generate(domain, projectName)
	if err != nil {
		fmt.Printf("Failed to generate domain: %v\n", err)
	}
}

func findProjectName() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current working directory: %w", err)
	}
	for {
		entities, err := os.ReadDir(cwd)
		if err != nil {
			return "", fmt.Errorf("failed to read directory: %w", err)
		}
		for _, entity := range entities {
			if entity.Name() == "go.mod" {
				data, err := os.ReadFile(filepath.Join(cwd, entity.Name()))
				if err != nil {
					return "", fmt.Errorf("failed to read go.mod file: %w", err)
				}
				goMod, err := modfile.Parse(filepath.Join(cwd, "go.mod"), data, nil)
				if err != nil {
					return "", fmt.Errorf("failed to parse go.mod: %w", err)
				}
				return goMod.Module.Mod.Path, nil
			}
		}
		if cwd == "/" {
			return "", fmt.Errorf("failed to find go.mod")
		}
		cwd = filepath.Dir(cwd)
	}
}

func generate(domain, projectName string) error {
	params := map[string]string{
		"Domain":        domain,
		"TitleDomain":   strings.ToUpper(domain[:1]) + domain[1:],
		"PluralsDomain": english.PluralWord(2, domain, ""),
		"Project":       projectName,
	}

	path := filepath.Join("internal", "domain", domain)
	sources := map[string]string{
		filepath.Join("templates", "model", "entity.go.tpl"):      filepath.Join(path, "model", domain+".go"),
		filepath.Join("templates", "model", "errors.go.tpl"):      filepath.Join(path, "model", "errors.go"),
		filepath.Join("templates", "repository", "entity.go.tpl"): filepath.Join(path, "repository", domain+".go"),
		filepath.Join("templates", "service", "entity.go.tpl"):    filepath.Join(path, "service", domain+".go"),
		filepath.Join("templates", "migration.sql.tpl"):           filepath.Join(path, fmt.Sprintf("%s_create_%s_table.sql", time.Now().Format("20060102150405"), params["PluralsDomain"])),
		filepath.Join("templates", "di.go.tpl"):                   filepath.Join(path, "di.go"),
	}

	for source, target := range sources {
		if err := write(source, target, params); err != nil {
			return err
		}
	}

	return nil
}

func write(source, target string, params any) error {
	tpl, err := template.ParseFS(tpls, source)
	if err != nil {
		return fmt.Errorf("failed to parse template %s: %w", source, err)
	}
	if _, err = os.Stat(target); !errors.Is(err, os.ErrNotExist) {
		return nil
	}
	err = os.MkdirAll(filepath.Dir(target), 0755)
	if err != nil {
		return fmt.Errorf("failed to create directory %s: %w", filepath.Dir(target), err)
	}
	fh, err := os.Create(target)
	if err != nil {
		return fmt.Errorf("failed to create %s: %w", target, err)
	}
	defer fh.Close()
	return tpl.Execute(fh, params)
}
