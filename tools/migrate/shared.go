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

// CheckShared refuses a version two sources ship with different contents: the
// sources share one bookkeeping table, so goose applies whichever runs first and
// skips the other without a word.
func CheckShared(sources map[string]fs.FS) error {
	type shipped struct {
		source string
		body   []byte
	}
	seen := map[string]shipped{}
	var conflicts []string
	for _, name := range slices.Sorted(maps.Keys(sources)) {
		entries, err := fs.ReadDir(sources[name], "migrations")
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		for _, entry := range entries {
			version, _, found := strings.Cut(entry.Name(), "_")
			if entry.IsDir() || !found {
				continue
			}
			body, err := fs.ReadFile(sources[name], path.Join("migrations", entry.Name()))
			if err != nil {
				return fmt.Errorf("%s: %w", name, err)
			}
			first, taken := seen[version]
			if !taken {
				seen[version] = shipped{source: name, body: body}
				continue
			}
			if !bytes.Equal(first.body, body) {
				conflicts = append(conflicts, fmt.Sprintf("%s ships in %s and %s with different contents", version, first.source, name))
			}
		}
	}
	if len(conflicts) > 0 {
		return fmt.Errorf("a shared version is applied from whichever source runs first, and the other is skipped:\n  %s",
			strings.Join(conflicts, "\n  "))
	}
	return nil
}
