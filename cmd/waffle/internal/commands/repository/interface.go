package repository

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/tools/imports"

	"gitlab.com/gorib/waffle/cmd/waffle/internal/pkg/command"
)

const serviceFile = "service.go"

// appendInterface puts the entity's repository interface where a domain keeps
// them: in service.go, above the management that takes them.
func appendInterface(c *command.Command, domain, entity string, params map[string]string) error {
	dir := filepath.Join("domain", domain, "service")
	declared := command.Title(entity) + "Repository"
	if where, found := command.Declares(dir, declared); found {
		command.KeptAs("service."+declared, where)
		return nil
	}

	// Not *.go.tpl: that suffix means a whole file, and this is a block appended
	// to one. TestTemplatesRenderToGo holds the distinction.
	block, err := c.Render(filepath.Join("templates", "service", "interface.gofrag.tpl"), params)
	if err != nil {
		return err
	}

	target := filepath.Join(dir, serviceFile)
	body, err := os.ReadFile(target)
	if err != nil {
		// No service.go to append to. Guessing where the block belongs would be
		// worse than saying so.
		command.ReportBlock(fmt.Sprintf("add to %s by hand:", filepath.ToSlash(target)), string(block))
		return nil
	}

	merged := insertBeforeFirstFunc(body, block)
	// The block needs context, criteria and the domain model, and which of them
	// the file already has depends on how it was scaffolded.
	formatted, err := imports.Process(target, merged, nil)
	if err != nil {
		return fmt.Errorf("adding %s to %s: %w", declared, target, err)
	}
	if err := os.WriteFile(target, formatted, 0644); err != nil {
		return err
	}
	command.ReportAction("added " + declared + " to " + filepath.ToSlash(target))
	return nil
}

// Above the first declaration that is not a type: interfaces at the top, the
// management below them, which is the order every domain here already has.
func insertBeforeFirstFunc(body, block []byte) []byte {
	lines := bytes.Split(body, []byte("\n"))
	at := len(lines)
	for i, line := range lines {
		if bytes.HasPrefix(line, []byte("func ")) {
			at = i
			break
		}
	}
	var out bytes.Buffer
	out.Write(bytes.Join(lines[:at], []byte("\n")))
	out.WriteString("\n")
	out.Write(block)
	out.WriteString("\n")
	out.Write(bytes.Join(lines[at:], []byte("\n")))
	return out.Bytes()
}
