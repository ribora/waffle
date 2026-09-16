package domain

import (
	"context"
	"embed"
	"fmt"
	"os/exec"
	"slices"

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
	info, err := exec.Command("make", ".tidy", "generate").CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %v", err, string(info))
	}
	return nil
}
