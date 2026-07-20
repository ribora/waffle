package grpc

import (
	"context"
	"embed"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"gitlab.com/gorib/waffle/cmd/waffle/internal/commands/converter"
	"gitlab.com/gorib/waffle/cmd/waffle/internal/pkg/command"
)

//go:embed templates/*
var templates embed.FS

func New() *cmd {
	return &cmd{command.New("make:domain:grpc", "Make grpc connector to the specified domain", templates, nil)}
}

type cmd struct {
	*command.Command
}

func (c *cmd) Run(ctx context.Context, args ...string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: waffle %s <domain>", c.Code())
	}
	domain := args[0]
	projectName, err := c.FindProjectName()
	if err != nil {
		return err
	}
	service := filepath.Base(projectName)

	cfg, err := c.ProjectConfig(projectName)
	if err != nil {
		return err
	}

	params := map[string]string{
		"Domain":    domain,
		"Project":   projectName,
		"Service":   service,
		"ApiImport": cfg.ApiImport,
	}

	path := filepath.Join("domain", domain)
	sources := map[string]string{
		filepath.Join("templates", "api", "service", "proto", "entity", "service.proto.tpl"): filepath.Join(cfg.ProtoDir, domain, "service.proto"),
		filepath.Join("templates", "service.go.tpl"):                                         filepath.Join(path, "cmd", "grpc", "service.go"),
		filepath.Join("templates", "errors.go.tpl"):                                          filepath.Join(path, "cmd", "grpc", "errors.go"),
		filepath.Join("templates", "di.go.tpl"):                                              filepath.Join("config", "di", "di_"+domain+"_grpc.go"),
	}

	if err := c.Write(sources, params); err != nil {
		return err
	}
	if cfg.Mapper == "goverter" {
		if err := converter.Generate(filepath.Join(path, "cmd", "grpc", "convert"), projectName); err != nil {
			return err
		}
	}
	if err := c.UncommentModule("gitlab.com/gorib/waffle/config/tools/grpc"); err != nil {
		return err
	}

	if strings.HasPrefix(filepath.Clean(cfg.ProtoDir), "..") {
		// pb is generated centrally (separate module) — can't build here, and
		// `make .tidy` would try to fetch the scaffolded pb import remotely.
		apiDir := filepath.Dir(filepath.Clean(cfg.ProtoDir))
		fmt.Printf(`Scaffolded gRPC handler for %q.
proto_dir (%s) is outside this module — pb is generated centrally, so this
command can't build it. Finish manually, in order:
  1. regenerate protobuf (e.g. make -C %s)
  2. go mod tidy
  3. go generate ./...
`, domain, cfg.ProtoDir, apiDir)
		return nil
	}

	// generate pb before tidy, else tidy fetches the not-yet-generated import remotely.
	if info, err := exec.Command("go", "generate", "./api/...").CombinedOutput(); err != nil {
		return fmt.Errorf("%v: %v", err, string(info))
	}
	if info, err := exec.Command("make", ".tidy", "generate").CombinedOutput(); err != nil {
		return fmt.Errorf("%v: %v", err, string(info))
	}
	return nil
}
