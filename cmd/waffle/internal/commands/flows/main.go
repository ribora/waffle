package flows

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

	"gitlab.com/gorib/waffle/cmd/waffle/internal/pkg/command"
	"gitlab.com/gorib/waffle/tools/flows"
)

var noTemplates embed.FS

func New() *cmd {
	return &cmd{base: command.New("flows",
		"Assemble a project's flows from its services' records and check the services against each other",
		noTemplates, nil).
		WithUsage("<root> <index.yaml>\n" +
			"  root is searched for waffle services the way ./... is, and taken as the whole system; index.yaml is\n" +
			"  the project's index, and the pages are written beside it. Fails on prose that lost its flow and on\n" +
			"  what the services' records leave unclosed")}
}

type cmd struct {
	base *command.Command
}

func (c *cmd) Code() string        { return c.base.Code() }
func (c *cmd) Description() string { return c.base.Description() }
func (c *cmd) Usage() string       { return c.base.Usage() }

func (c *cmd) Run(_ context.Context, args ...string) error {
	if err := c.base.Operands(args, 2, 2); err != nil {
		return err
	}
	root, index := strings.TrimSuffix(args[0], "..."), args[1]
	files, foreign, err := flows.Assemble(root, index)
	if err != nil {
		return err
	}
	for _, path := range foreign {
		command.ReportWarning("skipped " + path + ": it is not a specrig record")
	}
	stale := stalePages(filepath.Dir(index), files)
	unclosed := unclosed(files[index])
	if current, err := os.ReadFile(index); err == nil {
		for _, change := range flows.LinkChanges(current, files[index]) {
			command.ReportWarning(change)
		}
	}

	if err := os.MkdirAll(filepath.Dir(index), 0o755); err != nil {
		return err
	}
	for _, path := range slices.Sorted(maps.Keys(files)) {
		if err := os.WriteFile(path, files[path], 0o644); err != nil {
			return err
		}
	}
	for _, path := range stale {
		if err := os.Remove(path); err != nil {
			return err
		}
		command.ReportAction("removed " + path)
	}
	command.ReportAction(fmt.Sprintf("wrote %s and %d pages", index, len(files)-1))
	if orphaned(files[index]) {
		return fmt.Errorf("%s: some prose lost its flow — see orphaned", index)
	}
	if unclosed != "" {
		return fmt.Errorf("the services' records do not close — see system in %s:%s", index, unclosed)
	}
	return nil
}

// What in the index's system section fails: the records' findings, not what
// they only tell.
func unclosed(index []byte) string {
	var i flows.Index
	if yaml.Unmarshal(index, &i) != nil || i.System == nil || !i.System.Failed() {
		return ""
	}
	failing := *i.System
	failing.PatternOnly, failing.Loops, failing.Outside = nil, nil, nil
	return "\n" + failing.String()
}

// A page this command wrote before and would not write now.
func stalePages(dir string, files map[string][]byte) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		path := filepath.Join(dir, e.Name())
		if e.IsDir() || filepath.Ext(path) != ".md" || files[path] != nil {
			continue
		}
		if body, err := os.ReadFile(path); err == nil && flows.Generated(body) {
			out = append(out, path)
		}
	}
	return out
}

func orphaned(index []byte) bool { return bytes.Contains(index, []byte("\norphaned:")) }
