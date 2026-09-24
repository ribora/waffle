package agents

import (
	"fmt"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"gitlab.com/gorib/waffle/cmd/waffle/internal/pkg/command"
)

// AGENTS.md renders this table and `waffle agents:guard` enforces it — one copy,
// so the two cannot disagree. It covers the layout waffle scaffolds; anything a
// repository lays out for itself is an area in waffle.yaml.
type Mode struct {
	Name   string
	Allows string // for the table in AGENTS.md
	Brief  string // what `waffle agents:mode` prints on entering it
	allow  func(rel string) bool
}

const NoMode = "none"

var modes = []Mode{
	{
		Name:   NoMode,
		Allows: "anything but the platform's files",
		Brief:  "No mode is on. Everything but the platform's files may be edited; a Rule #11 task has not started.",
		allow:  func(rel string) bool { return !platform(rel) },
	},
	{
		Name:   "spec",
		Allows: "`tests/feature/**.feature`",
		Brief: "Rule #11 steps 0–2: \"When the code came first\", \"Requirements in, specification out\", " +
			"\"Writing the cases\" and \"Verification, then approval\". Write the cases and the delta list, " +
			"hand them over, stop.",
		allow: feature,
	},
	{
		Name:   "steps",
		Allows: "`tests/**` except `tests/internal/` and `tests/feature/` — the specification is frozen",
		Brief: "Rule #11 step 3: \"Steps and fixtures\". The specification was approved by switching to this mode " +
			"and is frozen: a correction to a case is a return to spec mode. Bind the cases, run them red, stop.",
		allow: func(rel string) bool { return under(rel, "tests") && !feature(rel) && !platform(rel) },
	},
	{
		Name:   "redgreen",
		Allows: "production code — anything outside `tests/` but the platform's files",
		Brief: "Rule #11 step 4: \"Red-green\" and \"Changing an approved specification\". " +
			"The specification and the steps are frozen. Make the cases green, stop.",
		allow: func(rel string) bool { return !under(rel, "tests") && !platform(rel) },
	},
	{
		Name:   "platform",
		Allows: "the platform's files, and production code for a seam",
		Brief: "Platform mode: you work for the platform owner — `waffle.yaml`, `tests/internal/`, the areas " +
			"waffle.yaml opens to it, and production code where a seam is missing. Not the cases and not the steps.",
		allow: func(rel string) bool { return platform(rel) || !under(rel, "tests") },
	},
}

// An Area is a named set of paths. Grant names the modes the human may open it
// in by name. Modes, when set, takes the paths out of the mode table: they are
// open in exactly those modes, and in none if it is empty. Nil Modes leaves the
// table in charge, so a grant only adds.
type Area struct {
	Name  string
	Paths []string
	Modes []string
	Grant []string
}

// Defaults, so a service works without configuring anything; waffle.yaml
// replaces one by naming it.
var defaultAreas = []Area{
	{Name: "model", Paths: []string{"domain/*/model/"}, Grant: []string{"steps"}},
	{Name: "gomod", Paths: []string{"go.mod", "go.sum"}, Modes: []string{"platform"}, Grant: []string{"redgreen"}},
}

// For the table in AGENTS.md.
func (a Area) PathList() string { return "`" + strings.Join(a.Paths, "`, `") + "`" }
func (a Area) ModeList() string {
	if a.Modes == nil {
		return "as the table says"
	}
	return orNone(a.Modes)
}
func (a Area) GrantList() string {
	if len(a.Grant) == 0 {
		return "—"
	}
	return strings.Join(a.Grant, ", ")
}

func LookupMode(name string) (Mode, error) {
	for _, m := range modes {
		if m.Name == name {
			return m, nil
		}
	}
	return Mode{}, fmt.Errorf("unknown mode %q (have: %s)", name, strings.Join(modeNames(), ", "))
}

func modeNames() []string {
	names := make([]string, 0, len(modes))
	for _, m := range modes {
		names = append(names, m.Name)
	}
	return names
}

// Areas merges waffle.yaml over the defaults, sorted by name.
func Areas(cfg command.AgentsConfig) ([]Area, error) {
	byName := map[string]Area{}
	for _, a := range defaultAreas {
		byName[a.Name] = a
	}
	for name, a := range cfg.Areas {
		if len(a.Paths) == 0 {
			return nil, fmt.Errorf("waffle.yaml: agents.areas.%s names no paths", name)
		}
		for _, m := range append(slices.Clone(a.Modes), a.Grant...) {
			if _, err := LookupMode(m); err != nil {
				return nil, fmt.Errorf("waffle.yaml: agents.areas.%s: %w", name, err)
			}
		}
		byName[name] = Area{Name: name, Paths: a.Paths, Modes: a.Modes, Grant: a.Grant}
	}
	areas := make([]Area, 0, len(byName))
	for _, a := range byName {
		areas = append(areas, a)
	}
	sort.Slice(areas, func(i, j int) bool { return areas[i].Name < areas[j].Name })
	return areas, nil
}

type Verdict int

const (
	Allowed Verdict = iota
	Refused
	Unclaimed // outside the service, and no area of it names the path
)

// Rules answers for one service: root is its resolved directory.
type Rules struct {
	Root  string
	Areas []Area
}

func (r Rules) Check(mode Mode, abs string, granted []string) (Verdict, string) {
	abs = filepath.Clean(abs)
	rel, inside := within(r.Root, abs)
	if inside && rel == "CONTRIBUTING.md" {
		return Refused, "CONTRIBUTING.md belongs to the humans who own the service; no mode allows it"
	}
	shown := rel
	if !inside {
		shown = abs
	}

	a, claimed := r.area(abs)
	if !claimed && !inside {
		return Unclaimed, ""
	}
	if claimed && slices.Contains(a.Grant, mode.Name) && slices.Contains(granted, a.Name) {
		return Allowed, ""
	}

	var ways []string
	switch {
	case claimed && a.Modes != nil:
		if slices.Contains(a.Modes, mode.Name) {
			return Allowed, ""
		}
		for _, m := range a.Modes {
			if m != mode.Name && m != NoMode {
				ways = append(ways, m+" mode")
			}
		}
	case inside:
		if mode.allow(rel) {
			return Allowed, ""
		}
		for _, other := range modes {
			if other.Name != NoMode && other.Name != mode.Name && other.allow(rel) {
				ways = append(ways, other.Name+" mode")
			}
		}
	}
	if claimed {
		for _, m := range a.Grant {
			ways = append(ways, fmt.Sprintf("the %s grant in %s mode", a.Name, m))
		}
	}
	if len(ways) == 0 {
		return Refused, fmt.Sprintf("%s is in the %s area, which no mode opens", shown, a.Name)
	}
	return Refused, refusal(mode, shown, ways)
}

func refusal(mode Mode, shown string, ways []string) string {
	if mode.Name == NoMode {
		return fmt.Sprintf("%s is edited only in %s — stop and ask the human for it", shown, strings.Join(ways, ", or "))
	}
	return fmt.Sprintf("%s mode does not allow %s; %s does — stop and ask the human for it",
		mode.Name, shown, strings.Join(ways, ", or "))
}

func (r Rules) area(abs string) (Area, bool) {
	for _, a := range r.Areas {
		for _, p := range a.Paths {
			if matches(filepath.Join(r.Root, p), strings.HasSuffix(p, "/"), abs) {
				return a, true
			}
		}
	}
	return Area{}, false
}

// A pattern ending in a slash is a directory, and matches everything below it.
func matches(pattern string, dir bool, abs string) bool {
	if !dir {
		ok, _ := filepath.Match(pattern, abs)
		return ok
	}
	depth := len(strings.Split(filepath.ToSlash(pattern), "/"))
	parts := strings.Split(filepath.ToSlash(abs), "/")
	if len(parts) <= depth {
		return false
	}
	ok, _ := filepath.Match(pattern, filepath.FromSlash(strings.Join(parts[:depth], "/")))
	return ok
}

func within(root, abs string) (string, bool) {
	rel, err := filepath.Rel(root, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return path.Clean(filepath.ToSlash(rel)), true
}

func platform(rel string) bool {
	return rel == "waffle.yaml" || under(rel, "tests/internal")
}

func feature(rel string) bool {
	return under(rel, "tests/feature") && strings.HasSuffix(rel, ".feature")
}

func under(rel, dir string) bool {
	return rel == dir || strings.HasPrefix(rel, dir+"/")
}
