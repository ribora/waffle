package command

import (
	"errors"
	"fmt"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config mirrors waffle.yaml.
type Config struct {
	ApiImport string      `yaml:"api_import"`
	ProtoDir  string      `yaml:"proto_dir"`
	Mapper    string      `yaml:"mapper"`
	Mocks     MocksConfig `yaml:"mocks"`
}

// MocksConfig is the mocks: group. Grouped rather than three top-level keys,
// because the top level of waffle.yaml is what a reader skims.
type MocksConfig struct {
	Dir string `yaml:"dir"`
	// Both are keyed by import path, so an entry maps onto one mockgen call.
	// Ignore is what --check accepts as classified without generating anything.
	Generate map[string][]string `yaml:"generate"`
	Ignore   map[string][]string `yaml:"ignore"`
}

// ProjectConfig loads waffle.yaml; a missing file is identical to all-defaults.
func (c *Command) ProjectConfig(projectName string) (Config, error) {
	root, err := findModuleRoot()
	if err != nil {
		return Config{}, err
	}

	var cfg Config
	fh, err := os.Open(filepath.Join(root, "waffle.yaml"))
	if err == nil {
		//nolint:errcheck
		defer fh.Close()
		dec := yaml.NewDecoder(fh)
		dec.KnownFields(true)
		if err := dec.Decode(&cfg); err != nil {
			return Config{}, fmt.Errorf("failed to parse waffle.yaml: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return Config{}, fmt.Errorf("failed to open waffle.yaml: %w", err)
	}

	if cfg.ApiImport == "" {
		cfg.ApiImport = projectName + "/api/pb"
	}
	if cfg.ProtoDir == "" {
		cfg.ProtoDir = "api/proto"
	}
	// goverter by default: it fails generation on a field it cannot map, so a
	// field added to a model cannot silently stop reaching the wire. A
	// hand-written mapper loses that check and is opt-in.
	if cfg.Mapper == "" {
		cfg.Mapper = "goverter"
	}
	// Under the test tree, because that is the only thing that imports mocks —
	// but deliberately not under tests/internal/: Go's internal rule would then
	// scope them to the tests/ subtree, and a table test living next to the
	// defect it pins (domain/x/service/x_test.go) could not import them at all.
	// That colocated test is exactly where a function-level defect belongs, so
	// the visibility has to stay module-wide.
	if cfg.Mocks.Dir == "" {
		cfg.Mocks.Dir = filepath.Join("tests", "mocks")
	}

	if cfg.Mapper != "dto" && cfg.Mapper != "goverter" {
		return Config{}, fmt.Errorf("waffle.yaml: mapper must be \"dto\" or \"goverter\", got %q", cfg.Mapper)
	}
	if err := cfg.Mocks.validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

// Listed reports whether an interface appears under either key, which is what
// --check goes by.
func (m MocksConfig) Listed(path, name string) bool {
	return slices.Contains(m.Generate[path], name) || slices.Contains(m.Ignore[path], name)
}

// A typo here would otherwise surface as a mockgen run failing much later,
// naming a package rather than the line that asked for it.
func (m MocksConfig) validate() error {
	for _, key := range []string{"generate", "ignore"} {
		listed := m.Generate
		if key == "ignore" {
			listed = m.Ignore
		}
		for path, names := range listed {
			if strings.TrimSpace(path) == "" {
				return fmt.Errorf("waffle.yaml: mocks.%s has an entry with an empty import path", key)
			}
			if len(names) == 0 {
				return fmt.Errorf("waffle.yaml: mocks.%s lists %s with no interfaces", key, path)
			}
			for _, name := range names {
				if !token.IsIdentifier(name) || !token.IsExported(name) {
					return fmt.Errorf("waffle.yaml: mocks.%s lists %q under %s, which is not an exported Go identifier",
						key, name, path)
				}
			}
		}
	}
	for path, names := range m.Generate {
		for _, name := range names {
			if slices.Contains(m.Ignore[path], name) {
				return fmt.Errorf("waffle.yaml: %s.%s is in both mocks.generate and mocks.ignore", path, name)
			}
		}
	}
	return nil
}
