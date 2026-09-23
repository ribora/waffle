// Command waffle-hook is the Claude Code side of Rule #11's modes. It keeps
// which mode the human switched each service to, per session, and asks the
// service's own waffle — `agents:mode`, `agents:guard` — everything else, so the
// rules come from the waffle version the service depends on.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type input struct {
	SessionID string `json:"session_id"`
	Cwd       string `json:"cwd"`
	Prompt    string `json:"prompt"`
	ToolInput struct {
		FilePath     string `json:"file_path"`
		NotebookPath string `json:"notebook_path"`
	} `json:"tool_input"`
}

func main() {
	if len(os.Args) != 2 {
		// Not 2: for PreToolUse that exit code would block the edit.
		fmt.Fprintln(os.Stderr, "usage: waffle-hook session-start|prompt|edit")
		os.Exit(1)
	}
	var in input
	_ = json.NewDecoder(os.Stdin).Decode(&in)
	h := hook{data: dataDir(), run: runWaffle}

	switch os.Args[1] {
	case "session-start":
		fmt.Print(h.sessionStart(in))
	case "prompt":
		fmt.Print(h.prompt(in))
	case "edit":
		if out := h.edit(in); out != nil {
			enc := json.NewEncoder(os.Stdout)
			enc.SetEscapeHTML(false)
			_ = enc.Encode(out)
		}
	}
}

func dataDir() string {
	if dir := os.Getenv("CLAUDE_PLUGIN_DATA"); dir != "" {
		return dir
	}
	return filepath.Join(os.TempDir(), "waffle-plugin")
}

// WAFFLE_BIN points every service at one binary, for trying a waffle that is
// not released yet.
func runWaffle(dir string, args ...string) (string, error) {
	var cmd *exec.Cmd
	script := filepath.Join(dir, "waffle")
	switch bin := os.Getenv("WAFFLE_BIN"); {
	case bin != "":
		cmd = exec.Command(bin, args...)
	case executable(script):
		cmd = exec.Command(script, args...)
	default:
		cmd = exec.Command("go", append([]string{"tool", "waffle"}, args...)...)
	}
	cmd.Dir = dir
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	return strings.TrimSpace(out.String()), err
}

func executable(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir() && info.Mode()&0o111 != 0
}

type verdict int

const (
	ok      verdict = iota
	refused         // waffle answered no, with a reason
	noModes         // the service's waffle predates agents:guard
	broken          // waffle did not run
)

// waffle prints "Error: <reason>" for a refusal and `Unknown command "x"` for a
// command it does not have; anything else is a failure to run at all.
func classify(out string, err error) (verdict, string) {
	switch {
	case err == nil:
		return ok, out
	case strings.HasPrefix(out, "Error: "):
		return refused, strings.TrimPrefix(out, "Error: ")
	case strings.Contains(out, "Unknown command"):
		return noModes, out
	case out != "":
		return broken, out
	default:
		return broken, err.Error()
	}
}
