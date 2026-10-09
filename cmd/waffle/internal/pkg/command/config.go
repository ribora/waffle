package command

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"gitlab.com/gorib/waffle/internal/gomod"
	"gitlab.com/gorib/waffle/replacements"
	"gitlab.com/gorib/waffle/surface"
)

// Config mirrors waffle.yaml.
type Config struct {
	ApiImport    string              `yaml:"api_import"`
	ProtoDir     string              `yaml:"proto_dir"`
	Mapper       string              `yaml:"mapper"`
	Replacements replacements.Config `yaml:"replacements"`
	Agents       AgentsConfig        `yaml:"agents"`
	Flows        surface.Config      `yaml:"flows"`
}

type AgentsConfig struct {
	Areas map[string]AreaConfig `yaml:"areas"`
}

// An area's paths are relative to the service and may leave it; a trailing
// slash means everything below.
type AreaConfig struct {
	Paths []string `yaml:"paths"`
	Modes []string `yaml:"modes"`
	Grant []string `yaml:"grant"`
}

// ProjectConfig loads waffle.yaml; a missing file is identical to all-defaults.
func (c *Command) ProjectConfig(projectName string) (Config, error) {
	root, err := gomod.Root()
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

	if cfg.Mapper != "dto" && cfg.Mapper != "goverter" {
		return Config{}, fmt.Errorf("waffle.yaml: mapper must be \"dto\" or \"goverter\", got %q", cfg.Mapper)
	}
	if err := cfg.Replacements.Validate(); err != nil {
		return Config{}, err
	}
	if err := cfg.Flows.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}
