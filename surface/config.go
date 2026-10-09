package surface

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Clients map[string]string `yaml:"clients"`
}

func (c Config) Validate() error {
	for client, host := range c.Clients {
		if strings.TrimSpace(client) == "" {
			return errors.New("waffle.yaml: flows.clients has a client with no key")
		}
		if strings.TrimSpace(host) == "" {
			return fmt.Errorf("waffle.yaml: flows.clients names no host for %s", client)
		}
	}
	return nil
}

type projectFile struct {
	Flows Config `yaml:"flows"`
}

func LoadConfig(dir string) (Config, error) {
	body, err := os.ReadFile(filepath.Join(dir, "waffle.yaml"))
	if errors.Is(err, os.ErrNotExist) {
		return Config{}, nil
	}
	if err != nil {
		return Config{}, err
	}
	// Not KnownFields: the rest of the file belongs to the generators.
	var file projectFile
	if err := yaml.Unmarshal(body, &file); err != nil {
		return Config{}, fmt.Errorf("%s: %w", filepath.Join(dir, "waffle.yaml"), err)
	}
	return file.Flows, file.Flows.Validate()
}
