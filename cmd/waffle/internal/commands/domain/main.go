package domain

import (
	"context"
	"embed"
	"fmt"

	"github.com/ribora/waffle/cmd/waffle/internal/commands/agents"
	"github.com/ribora/waffle/cmd/waffle/internal/pkg/command"
)

//go:embed templates/*
var templateFiles embed.FS

var templates = command.NewTemplates(templateFiles)

func New() *cmd {
	return &cmd{command.New("make:domain", "Make domain with entity, repository and management").
		WithUsage("[--no-repository] <domain>")}
}

type cmd struct {
	*command.Command
}

func (c *cmd) Run(ctx context.Context, args ...string) error {
	// The model is written either way — what the flag skips is the repository,
	// and with it the table and the modules that serve one.
	noRepository, args := command.TakeBool(args, "--no-repository")

	if err := c.Operands(args, 1, 1); err != nil {
		return err
	}
	domain := args[0]
	if err := command.ValidateName("domain", domain); err != nil {
		return err
	}
	p, err := command.OpenProject()
	if err != nil {
		return err
	}
	if !noRepository {
		if err := p.RequireModules(command.MigrationModules...); err != nil {
			return err
		}
	}

	if err := Generate(p, domain, !noRepository); err != nil {
		return err
	}
	if !noRepository {
		if err := p.EnableStorage(); err != nil {
			return err
		}
	}
	// Not `make generate`: until a transport reads the management, diogen rightly
	// refuses the container, so only the domain's own packages are generated.
	for _, step := range [][]string{
		{"make", ".tidy"},
		{"go", "generate", "./domain/" + domain + "/..."},
	} {
		if _, err := p.Run(step[0], step[1:]...); err != nil {
			return err
		}
	}
	if _, err := agents.Generate(p, true); err != nil {
		return err
	}
	reportUnread(domain)
	return nil
}

func reportUnread(domain string) {
	command.ReportBlock(
		fmt.Sprintf("%s is not in the container until something reads its management — make generate refuses it until then; add a consumer:", domain),
		fmt.Sprintf("waffle make:domain:http %[1]s\nwaffle make:domain:grpc %[1]s\nwaffle make:domain:scribus --driver <id> %[1]s <event>\nwaffle make:domain:command %[1]s <command>", domain),
	)
}
