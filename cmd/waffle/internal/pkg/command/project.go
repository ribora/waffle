package command

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gitlab.com/gorib/waffle/internal/gomod"
)

// Every path a Project takes or reports is relative to the module root, not to
// the working directory.
type Project struct {
	root   string
	module string
	config *Config
}

func OpenProject() (*Project, error) {
	root, err := gomod.Root()
	if err != nil {
		return nil, err
	}
	module, err := gomod.Path(root)
	if err != nil {
		return nil, err
	}
	return &Project{root: root, module: module}, nil
}

func (p *Project) Root() string {
	return p.root
}

func (p *Project) Module() string {
	return p.module
}

func (p *Project) Config() (Config, error) {
	if p.config == nil {
		config, err := loadConfig(p.root, p.module)
		if err != nil {
			return Config{}, err
		}
		p.config = &config
	}
	return *p.config, nil
}

func (p *Project) Path(elem ...string) string {
	return filepath.Join(append([]string{p.root}, elem...)...)
}

func (p *Project) Glob(pattern string) ([]string, error) {
	found, err := filepath.Glob(p.Path(pattern))
	if err != nil {
		return nil, err
	}
	for i, path := range found {
		if found[i], err = filepath.Rel(p.root, path); err != nil {
			return nil, err
		}
	}
	return found, nil
}

func (p *Project) Run(name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = p.root
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("%s: %w: %s", strings.Join(cmd.Args, " "), err, out)
	}
	return out, nil
}

// Before the first write: the files a command adds to a domain that is not there
// are a tree that does not build.
func (p *Project) RequireDomain(domain string) error {
	dir := filepath.Join("domain", domain)
	if _, err := os.Stat(p.Path(dir)); errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("%s does not exist — create the domain first: waffle make:domain %s", dir, domain)
	} else if err != nil {
		return err
	}
	return nil
}
