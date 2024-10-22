package command

import (
	"bufio"
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"gitlab.com/gorib/waffle/cmd/waffle/internal/pkg/plural"
)

func New(name, description string, templates embed.FS, funcs template.FuncMap) *Command {
	c := &Command{
		name:        name,
		description: description,
		templates:   templates,
		funcs: template.FuncMap{
			"plural": plural.Plural,
			"title": func(word string) string {
				return strings.ToUpper(word[:1]) + word[1:]
			},
		},
	}
	for n, f := range funcs {
		c.funcs[n] = f
	}
	return c
}

type Command struct {
	name        string
	description string
	templates   embed.FS
	funcs       template.FuncMap
}

func (c *Command) Code() string {
	return c.name
}

func (c *Command) Description() string {
	return c.description
}

func (c *Command) Write(sources map[string]string, params any) error {
	for source, target := range sources {
		if err := c.write(source, target, params); err != nil {
			return err
		}
	}
	return nil
}

func (c *Command) write(source, target string, params any) error {
	tpl, err := template.New(filepath.Base(source)).Funcs(c.funcs).ParseFS(c.templates, source)
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

func (c *Command) FindProjectName() (string, error) {
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
				return "", errors.New("go.mod is invalid")
			}
		}
		if cwd == "/" {
			return "", fmt.Errorf("failed to find go.mod")
		}
		cwd = filepath.Dir(cwd)
	}
}
