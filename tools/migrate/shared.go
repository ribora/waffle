package migrate

import (
	"bytes"
	"fmt"
	"io/fs"
	"maps"
	"path"
	"slices"
	"strings"
)

// CheckShared refuses a version two sources ship with different contents, as run
// does, and names the sources, which run has no names for.
func CheckShared(sources map[string]fs.FS) error {
	_, err := collect(sources)
	return err
}

type shipped struct {
	label  string
	source fs.FS
	entry  fs.DirEntry
	body   []byte
}

func collect(sources map[string]fs.FS) (map[string]shipped, error) {
	seen := map[string]shipped{}
	var conflicts []string
	for _, label := range slices.Sorted(maps.Keys(sources)) {
		entries, err := fs.ReadDir(sources[label], "migrations")
		if err != nil {
			return nil, fmt.Errorf("%s: %w", label, err)
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			key, _, versioned := strings.Cut(entry.Name(), "_")
			if !versioned {
				key = entry.Name()
			}
			body, err := fs.ReadFile(sources[label], path.Join("migrations", entry.Name()))
			if err != nil {
				return nil, fmt.Errorf("%s: %w", label, err)
			}
			first, taken := seen[key]
			if !taken {
				seen[key] = shipped{label: label, source: sources[label], entry: entry, body: body}
				continue
			}
			if versioned && !bytes.Equal(first.body, body) {
				conflicts = append(conflicts, fmt.Sprintf("%s ships in %s as %s and in %s as %s, with different contents",
					key, first.label, first.entry.Name(), label, entry.Name()))
			}
		}
	}
	if len(conflicts) > 0 {
		return nil, fmt.Errorf("a version runs once, so the sources sharing it must ship the same file:\n  %s",
			strings.Join(conflicts, "\n  "))
	}
	return seen, nil
}
