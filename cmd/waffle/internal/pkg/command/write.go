package command

import (
	"bytes"
	"errors"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

func (p *Project) Write(templates Templates, sources map[string]string, params any) error {
	type pair struct{ source, target string }

	pairs := make([]pair, 0, len(sources))
	for source, target := range sources {
		pairs = append(pairs, pair{source, target})
	}
	// Map order would shuffle the report between runs, and a report that reshuffles
	// is one nobody reads twice.
	slices.SortFunc(pairs, func(a, b pair) int { return strings.Compare(a.target, b.target) })

	var created, kept []string
	for _, pair := range pairs {
		wrote, err := p.write(templates, pair.source, pair.target, params)
		if err != nil {
			return err
		}
		if wrote {
			created = append(created, pair.target)
		} else {
			kept = append(kept, pair.target)
		}
	}
	reportCreated(created)
	reportKept(kept)
	return nil
}

func (p *Project) write(templates Templates, source, target string, params any) (created bool, err error) {
	path := p.Path(target)
	// An unreadable target used to count as "already there" and was skipped in
	// silence, which is the one case where skipping is wrong.
	switch _, err := os.Stat(path); {
	case err == nil:
		return false, nil
	case !errors.Is(err, os.ErrNotExist):
		return false, fmt.Errorf("failed to stat %s: %w", target, err)
	}

	out, err := templates.Render(source, params)
	if err != nil {
		return false, err
	}
	// Rendering straight to the file made the template the source of formatting,
	// and two copies of one template had already drifted apart on indentation.
	if filepath.Ext(target) == ".go" {
		if out, err = format.Source(out); err != nil {
			return false, fmt.Errorf("%s renders invalid Go: %w", source, err)
		}
	}
	if err = os.MkdirAll(filepath.Dir(path), os.ModePerm); err != nil {
		return false, fmt.Errorf("failed to create directory %s: %w", filepath.Dir(target), err)
	}
	if err = os.WriteFile(path, out, 0644); err != nil {
		return false, fmt.Errorf("failed to create %s: %w", target, err)
	}
	return true, nil
}

// For a file that is regenerated rather than scaffolded: one already holding body
// is not touched, so a run that changes nothing leaves no trace on disk.
func WriteIfChanged(path string, body []byte) (bool, error) {
	if current, err := os.ReadFile(path); err == nil && bytes.Equal(current, body) {
		return false, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		return false, err
	}
	return true, nil
}
