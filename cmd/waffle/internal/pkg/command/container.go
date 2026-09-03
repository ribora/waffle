package command

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Container is one generated di container, located by the //go:generate
// directive that builds it. The path is already written there, and a copy of it
// in waffle.yaml would be a second thing to keep in sync.
type Container struct {
	File    string // the generated file, relative to the working directory
	Dir     string // the directory holding it
	Package string // its package clause
}

// DiscoverContainers reads -output off every //go:generate line below the
// working directory that invokes diogen. A service with two containers gets two
// entries, which is why this returns a slice rather than the one path everyone
// happens to have.
func DiscoverContainers() ([]Container, error) {
	var found []Container
	seen := map[string]bool{}

	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case d.IsDir():
			if path != "." && skipDir(d.Name()) {
				return fs.SkipDir
			}
			return nil
		case !strings.HasSuffix(path, ".go"):
			return nil
		}

		directives, err := diogenDirectives(path)
		if err != nil {
			return err
		}
		for _, fields := range directives {
			output, ok := flagValue(fields, "-output")
			if !ok {
				continue
			}
			output = filepath.Clean(output)
			if seen[output] {
				continue
			}
			seen[output] = true

			pkg, ok := flagValue(fields, "-pkg")
			if !ok {
				// diogen falls back to the package of its input, which only
				// coincides with the output directory when the two are the same.
				pkg = filepath.Base(filepath.Dir(output))
			}
			found = append(found, Container{File: output, Dir: filepath.Dir(output), Package: pkg})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(found) == 0 {
		return nil, errors.New("no di container found: nothing below the current directory carries a " +
			"//go:generate line invoking diogen with -output — run this from the root of a waffle service")
	}
	slices.SortFunc(found, func(a, b Container) int { return strings.Compare(a.File, b.File) })
	return found, nil
}

func skipDir(name string) bool {
	return name == "vendor" || name == "testdata" || strings.HasPrefix(name, ".")
}

func diogenDirectives(path string) ([][]string, error) {
	fh, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}
	//nolint:errcheck
	defer fh.Close()

	var out [][]string
	sc := bufio.NewScanner(fh)
	for sc.Scan() {
		rest, found := strings.CutPrefix(sc.Text(), "//go:generate")
		if !found {
			continue
		}
		fields := strings.Fields(rest)
		// "go tool diogen", "diogen", or an absolute path to it.
		if slices.ContainsFunc(fields, func(f string) bool {
			return f == "diogen" || strings.HasSuffix(f, "/diogen")
		}) {
			out = append(out, fields)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}
	return out, nil
}

// Both spellings: the scaffolded directive writes "-output path", a hand-edited
// one may well write "-output=path".
func flagValue(fields []string, name string) (string, bool) {
	for i, f := range fields {
		switch {
		case f == name && i+1 < len(fields):
			return fields[i+1], true
		case strings.HasPrefix(f, name+"="):
			return strings.TrimPrefix(f, name+"="), true
		}
	}
	return "", false
}
