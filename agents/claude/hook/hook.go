package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
)

const none = "none"

type hook struct {
	data string
	run  func(dir string, args ...string) (string, error)
}

// One mode holds for every service the task names; a service it stops naming is
// back to none.
type state struct {
	Mode string `json:"mode"`
	// The services the last command named by path. A command naming none acts
	// on them, so a path is typed once per task — there is no completion for it.
	Services []string        `json:"services"`
	Granted  []string        `json:"granted"` // areas granted on top of the mode; a switch clears them
	Warned   map[string]bool `json:"warned"`  // services already told their waffle has no modes

	// A session recorded before one mode per session; read once and dropped.
	OldModes  map[string]string   `json:"modes,omitempty"`
	OldGrants map[string][]string `json:"grants,omitempty"`
}

func (s state) modeOf(svc string) string {
	if s.Mode != "" && slices.Contains(s.Services, svc) {
		return s.Mode
	}
	return none
}

func (h hook) sessionStart(in input) string {
	var b strings.Builder
	b.WriteString("waffle plugin active: Rule #11 modes are enforced for waffle services. " +
		"An edit the mode does not allow is refused, naming the mode or grant that would allow it. " +
		"Only the human switches the mode, by typing /waffle:<mode> [./path/to/service…], " +
		"and only the human grants an area, by typing /waffle:allow <area>; nothing you run does either.\n")
	st := h.load(in.SessionID)
	if st.Mode != "" && len(st.Services) > 0 {
		fmt.Fprintf(&b, "Current mode: %s, for %s.\n", st.Mode, h.names(in.Cwd, st.Services))
	}
	if len(st.Granted) > 0 {
		fmt.Fprintf(&b, "Granted: %s.\n", strings.Join(st.Granted, ", "))
	}
	return b.String()
}

func (h hook) prompt(in input) string {
	fields := strings.Fields(in.Prompt)
	if len(fields) == 0 || !strings.HasPrefix(fields[0], "/waffle:") {
		return ""
	}
	command := strings.TrimPrefix(fields[0], "/waffle:")

	var named, rest []string
	for _, arg := range fields[1:] {
		if svc := resolveService(in.Cwd, arg); svc != "" {
			named = append(named, svc)
		} else {
			rest = append(rest, arg)
		}
	}
	st := h.load(in.SessionID)
	services := named
	switch {
	case len(services) > 0:
	case len(st.Services) > 0:
		services = st.Services
	default:
		if svc := serviceOf(in.Cwd); svc != "" {
			services = []string{svc}
		} else {
			return fmt.Sprintf("Nothing was switched: no waffle service is named, none was named earlier in "+
				"this session, and %s is not inside one. Tell the human to name one by its path, once: "+
				"/waffle:%s ./path/to/service.\n", in.Cwd, command)
		}
	}

	if command == "allow" {
		out := h.allow(in, &st, services, rest)
		h.save(in.SessionID, st)
		return out
	}

	var b strings.Builder
	switched := false
	for _, svc := range services {
		name := display(in.Cwd, svc)
		v, text := classify(h.run(svc, "agents:mode", command))
		switch v {
		case ok:
			switched = true
			fmt.Fprintf(&b, "%s:\n%s\n\n", name, text)
		case noModes:
			fmt.Fprintf(&b, "%s: its waffle has no modes yet, so edits there are not checked — upgrade waffle in it.\n\n", name)
		default:
			fmt.Fprintf(&b, "%s was not switched: %s\n\n", name, text)
		}
	}
	if !switched {
		return b.String()
	}
	st.Mode, st.Services, st.Granted = command, services, nil
	h.save(in.SessionID, st)
	return fmt.Sprintf("The human switched to %s mode, for %s; every other service is in none, "+
		"and nothing is granted.\n\n%s", command, h.names(in.Cwd, services), b.String())
}

// allow answers /waffle:allow [area]: with no name it lists the areas the mode
// takes as grants; with one, each service's waffle checks it belongs there.
func (h hook) allow(in input, st *state, services, rest []string) string {
	mode := st.Mode
	if mode == "" {
		mode = none
	}
	var b strings.Builder
	granted := false
	for _, svc := range services {
		name := display(in.Cwd, svc)
		args := []string{"agents:grant", mode}
		if len(rest) > 0 {
			args = append(args, rest[0])
		}
		v, text := classify(h.run(svc, args...))
		switch v {
		case ok:
			granted = granted || len(rest) > 0
			fmt.Fprintf(&b, "%s, in %s mode:\n%s\n\n", name, mode, text)
		case noModes:
			fmt.Fprintf(&b, "%s: its waffle has no grants yet — upgrade waffle in it.\n\n", name)
		default:
			fmt.Fprintf(&b, "%s: %s\n\n", name, text)
		}
	}
	if !granted {
		return b.String()
	}
	if !slices.Contains(st.Granted, rest[0]) {
		st.Granted = append(st.Granted, rest[0])
	}
	return fmt.Sprintf("The human granted %s, on top of %s mode, until the mode changes.\n\n%s",
		rest[0], mode, b.String())
}

type decision struct {
	HookSpecificOutput *denial `json:"hookSpecificOutput,omitempty"`
	SystemMessage      string  `json:"systemMessage,omitempty"`
}

type denial struct {
	HookEventName            string `json:"hookEventName"`
	PermissionDecision       string `json:"permissionDecision"`
	PermissionDecisionReason string `json:"permissionDecisionReason"`
}

// A nil answer defers to the normal permission flow; this hook only ever
// refuses, it never grants.
func (h hook) edit(in input) *decision {
	path := in.ToolInput.FilePath
	if path == "" {
		path = in.ToolInput.NotebookPath
	}
	if path == "" {
		return nil
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(in.Cwd, path)
	}
	st := h.load(in.SessionID)

	// Inside a service, that service decides. Outside every service, each one the
	// task names is asked whether one of its areas claims the path.
	asked := st.Services
	if svc := serviceOf(filepath.Dir(path)); svc != "" {
		asked = []string{svc}
	}
	var warn []string
	for _, svc := range asked {
		args := []string{"agents:guard", st.modeOf(svc), path}
		if st.modeOf(svc) != none {
			for _, g := range st.Granted {
				args = append(args, "--grant", g)
			}
		}
		switch v, text := classify(h.run(svc, args...)); v {
		case refused:
			how := ". The human switches with /waffle:<mode>" + argument(in.Cwd, svc)
			if strings.Contains(text, " grant ") {
				how += ", and grants with /waffle:allow <area>"
			}
			return &decision{HookSpecificOutput: &denial{
				HookEventName:            "PreToolUse",
				PermissionDecision:       "deny",
				PermissionDecisionReason: text + how + ".",
			}}
		case ok:
			if !strings.HasPrefix(text, "unclaimed") {
				return nil
			}
		case noModes, broken:
			if !st.Warned[svc] {
				st.Warned[svc] = true
				if v == noModes {
					warn = append(warn, display(in.Cwd, svc)+": its waffle has no modes yet, so edits there are not checked — upgrade waffle in it.")
				} else {
					warn = append(warn, display(in.Cwd, svc)+": waffle did not run ("+text+"), so edits there are not checked.")
				}
			}
		}
	}
	if len(warn) == 0 {
		return nil
	}
	h.save(in.SessionID, st)
	return &decision{SystemMessage: strings.Join(warn, " ")}
}

func (h hook) names(cwd string, services []string) string {
	shown := make([]string, 0, len(services))
	for _, svc := range services {
		shown = append(shown, display(cwd, svc))
	}
	return strings.Join(shown, ", ")
}

var unsafe = regexp.MustCompile(`[^A-Za-z0-9_-]`)

func (h hook) file(session string) string {
	return filepath.Join(h.data, "sessions", unsafe.ReplaceAllString(session, "_")+".json")
}

func (h hook) load(session string) state {
	st := state{}
	if body, err := os.ReadFile(h.file(session)); err == nil {
		_ = json.Unmarshal(body, &st)
	}
	if st.Warned == nil {
		st.Warned = map[string]bool{}
	}
	if st.Mode == "" && len(st.OldModes) > 0 {
		var svcs []string
		for svc := range st.OldModes {
			svcs = append(svcs, svc)
		}
		sort.Strings(svcs)
		st.Mode = st.OldModes[svcs[0]]
		if len(st.Services) == 0 {
			st.Services = svcs
		}
		for _, granted := range st.OldGrants {
			for _, g := range granted {
				if !slices.Contains(st.Granted, g) {
					st.Granted = append(st.Granted, g)
				}
			}
		}
	}
	st.OldModes, st.OldGrants = nil, nil
	return st
}

// Written through a rename: two hooks of one session can run at once.
func (h hook) save(session string, st state) {
	path := h.file(session)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	body, err := json.Marshal(st)
	if err != nil {
		return
	}
	tmp := fmt.Sprintf("%s.%d", path, os.Getpid())
	if os.WriteFile(tmp, body, 0o644) == nil {
		_ = os.Rename(tmp, path)
	}
}

// serviceOf walks up from dir, which may not exist yet, to the nearest
// directory holding waffle.yaml.
func serviceOf(dir string) string {
	for d := filepath.Clean(dir); ; {
		if _, err := os.Stat(filepath.Join(d, "waffle.yaml")); err == nil {
			return resolve(d)
		}
		parent := filepath.Dir(d)
		if parent == d {
			return ""
		}
		d = parent
	}
}

// An argument names a service only by its path: waffle does not know how a
// repository lays its services out. Anything else is the task's text.
func resolveService(cwd, arg string) string {
	path := arg
	if !filepath.IsAbs(path) {
		path = filepath.Join(cwd, arg)
	}
	if _, err := os.Stat(filepath.Join(path, "waffle.yaml")); err == nil {
		return resolve(path)
	}
	return ""
}

func resolve(dir string) string {
	if r, err := filepath.EvalSymlinks(dir); err == nil {
		return r
	}
	return dir
}

// What follows /waffle:<mode> to name svc: nothing when the session runs in it.
func argument(cwd, svc string) string {
	rel, err := filepath.Rel(resolve(cwd), svc)
	switch {
	case err != nil, strings.HasPrefix(rel, ".."):
		return " " + svc
	case rel == ".":
		return ""
	default:
		return " ./" + filepath.ToSlash(rel)
	}
}

func display(cwd, svc string) string {
	if rel, err := filepath.Rel(resolve(cwd), svc); err == nil && !strings.HasPrefix(rel, "..") {
		if rel == "." {
			return "this service"
		}
		return rel
	}
	return svc
}
