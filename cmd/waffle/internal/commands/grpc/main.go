package grpc

import (
	"context"
	"embed"
	"fmt"
	"path/filepath"

	"gitlab.com/gorib/waffle/cmd/waffle/internal/pkg/command"
)

//go:embed templates/*
var templates embed.FS

func New() *cmd {
	return &cmd{command.New("make:grpc", "Make grpc connector to the specified domain", templates, nil)}
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

	params := map[string]string{
		"Domain":  domain,
		"Project": projectName,
		"Service": service,
	}

	sources := map[string]string{
		filepath.Join("templates", "api", "service", "pb", "entity", "service.proto.tpl"): filepath.Join("api", service, "pb", domain, "service.proto"),
		filepath.Join("templates", "entity", "service.go.tpl"):                            filepath.Join("tools", "grpc", domain, "service.go"),
		filepath.Join("templates", "di.go.tpl"):                                           filepath.Join("config", "di_grpc_"+domain+".go"),
	}

	return c.Write(sources, params)
}
