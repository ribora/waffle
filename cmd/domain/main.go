package main

import (
	"bufio"
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"
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
				goMod, err := os.OpenFile(filepath.Join(cwd, "go.mod"), os.O_RDONLY, os.ModePerm)
				if err != nil {
					return "", fmt.Errorf("failed to open go.mod file: %w", err)
				}
				defer goMod.Close()

				sc := bufio.NewScanner(goMod)
				for sc.Scan() {
					line := sc.Text()
					if pkg, found := strings.CutPrefix(line, "module "); found {
						return pkg, nil
					}
				}
				if err := sc.Err(); err != nil {
					return "", fmt.Errorf("failed to read go.mod file: %w", err)
				}
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
		"Domain":  domain,
		"Project": projectName,
	}

	funcs := template.FuncMap{
		"plural": Plural,
		"title": func(word string) string {
			return strings.ToUpper(word[:1]) + word[1:]
		},
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
		if err := write(source, target, params, funcs); err != nil {
			return err
		}
	}

	return nil
}

func write(source, target string, params any, funcs template.FuncMap) error {
	tpl, err := template.New(filepath.Base(source)).Funcs(funcs).ParseFS(tpls, source)
	if err != nil {
		return fmt.Errorf("failed to parse template %s: %w", source, err)
	}
	if _, err = os.Stat(target); !errors.Is(err, os.ErrNotExist) {
		return nil
	}
	err = os.MkdirAll(filepath.Dir(target), os.ModePerm)
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
