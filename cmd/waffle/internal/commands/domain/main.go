package domain

import (
	"context"
	"embed"
	"fmt"
	"os/exec"
	"slices"

	"gitlab.com/gorib/waffle/cmd/waffle/internal/commands/agents"
	"gitlab.com/gorib/waffle/cmd/waffle/internal/pkg/command"
)

//go:embed templates/*
var templates embed.FS

func New() *cmd {
	return &cmd{command.New("make:domain", "Make domain with entity, repository and management", templates, nil).
		WithUsage("[--no-repository] <domain>")}
}

type cmd struct {
	*command.Command
}

func (c *cmd) Run(ctx context.Context, args ...string) error {
	// The model is written either way — what the flag skips is the repository,
	// and with it the table and the modules that serve one.
	noRepository := false
	if i := slices.Index(args, "--no-repository"); i >= 0 {
		noRepository = true
		args = append(args[:i], args[i+1:]...)
	}

	if err := c.Operands(args, 1, 1); err != nil {
		return err
	}
	domain := args[0]
	if err := command.ValidateName("domain", domain); err != nil {
		return err
	}
	projectName, err := c.FindProjectName()
	if err != nil {
		return err
	}
	// Before the first write: eleven files and a go.mod edit stand between here
	// and UncommentModule.
	if !noRepository {
		if err := c.RequireModules(); err != nil {
			return err
		}
	}

	if err := Generate(domain, projectName, !noRepository); err != nil {
		return err
	}
	if !noRepository {
		if err := c.EnableStorage(); err != nil {
			return err
		}
	}
	// Not `make generate`: until a transport reads the management, diogen rightly
	// refuses the container, so only the domain's own packages are generated.
	for _, step := range [][]string{
		{"make", ".tidy"},
		{"go", "generate", "./domain/" + domain + "/..."},
	} {
		if info, err := exec.Command(step[0], step[1:]...).CombinedOutput(); err != nil {
			return fmt.Errorf("%v: %v", err, string(info))
		}
	}
	if _, err := agents.Generate(projectName, true); err != nil {
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
