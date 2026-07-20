package command

import (
	"errors"
	"fmt"
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

// ProjectConfig loads waffle.yaml; a missing file is identical to all-defaults.
func (c *Command) ProjectConfig(projectName string) (Config, error) {
	root, err := findModuleRoot()
	if err != nil {
		return Config{}, err
	}

	var cfg Config
	fh, err := os.Open(filepath.Join(root, "waffle.yaml"))
	if err == nil {
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
	if cfg.Mapper == "" {
		cfg.Mapper = "dto"
	}

	if cfg.Mapper != "dto" && cfg.Mapper != "goverter" {
		return Config{}, fmt.Errorf("waffle.yaml: mapper must be \"dto\" or \"goverter\", got %q", cfg.Mapper)
	}

	return cfg, nil
}
