package command

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config mirrors waffle.yaml.
type Config struct {
	ApiImport string `yaml:"api_import"`
	ProtoDir  string `yaml:"proto_dir"`
	Mapper    string `yaml:"mapper"`
}

func loadConfig(root, module string) (Config, error) {
	var cfg Config
	fh, err := os.Open(filepath.Join(root, "waffle.yaml"))
	if err == nil {
		//nolint:errcheck
		defer fh.Close()
		dec := yaml.NewDecoder(fh)
		dec.KnownFields(true)
		// io.EOF is yaml.v3's answer to a file with no document: empty, or comments only.
		if err := dec.Decode(&cfg); err != nil && !errors.Is(err, io.EOF) {
			return Config{}, fmt.Errorf("failed to parse waffle.yaml: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return Config{}, fmt.Errorf("failed to open waffle.yaml: %w", err)
	}

	if cfg.ApiImport == "" {
		cfg.ApiImport = module + "/api/pb"
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

	if cfg.Mapper != "dto" && cfg.Mapper != "goverter" {
		return Config{}, fmt.Errorf("waffle.yaml: mapper must be \"dto\" or \"goverter\", got %q", cfg.Mapper)
	}
	return cfg, nil
}
