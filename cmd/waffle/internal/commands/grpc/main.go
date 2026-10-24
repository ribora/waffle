package grpc

import (
	"context"
	"embed"
	"fmt"
	"path/filepath"
	"strings"

	"gitlab.com/gorib/waffle/cmd/waffle/internal/commands/converter"
	"gitlab.com/gorib/waffle/cmd/waffle/internal/pkg/command"
)

//go:embed templates/*
var templateFiles embed.FS

var templates = command.NewTemplates(templateFiles)

const module = "gitlab.com/gorib/waffle/config/tools/grpc"

func New() *cmd {
	return &cmd{command.New("make:domain:grpc", "Make grpc connector to the specified domain").
		WithUsage("<domain>")}
}

type cmd struct {
	*command.Command
}

func (c *cmd) Run(ctx context.Context, args ...string) error {
	if err := c.Operands(args, 1, 1); err != nil {
		return err
	}
	if err := command.ValidateName("domain", args[0]); err != nil {
		return err
	}
	domain := args[0]
	p, err := command.OpenProject()
	if err != nil {
		return err
	}
	if err := p.RequireDomain(domain); err != nil {
		return err
	}
	if err := p.RequireModules(module); err != nil {
		return err
	}
	cfg, err := p.Config()
	if err != nil {
		return err
	}

	params := map[string]string{
		"Domain":    domain,
		"Project":   p.Module(),
		"Service":   filepath.Base(p.Module()),
		"ApiImport": cfg.ApiImport,
	}

	path := filepath.Join("domain", domain)
	sources := map[string]string{
		filepath.Join("templates", "api", "service", "proto", "entity", "service.proto.tpl"): filepath.Join(cfg.ProtoDir, domain, "service.proto"),
		filepath.Join("templates", "service.go.tpl"):                                         filepath.Join(path, "cmd", "grpc", "service.go"),
		filepath.Join("templates", "errors.go.tpl"):                                          filepath.Join(path, "cmd", "grpc", "errors.go"),
		filepath.Join("templates", "di.go.tpl"):                                              filepath.Join("config", "di", "di_"+domain+"_grpc.go"),
	}

	if err := p.Write(templates, sources, params); err != nil {
		return err
	}
	if cfg.Mapper == "goverter" {
		if err := converter.Generate(p, filepath.Join(path, "cmd", "grpc", "convert")); err != nil {
			return err
		}
	}
	if err := p.EnableModule(module); err != nil {
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
	if _, err := p.Run("go", "generate", "./api/..."); err != nil {
		return err
	}
	_, err = p.Run("make", ".tidy", "generate")
	return err
}
