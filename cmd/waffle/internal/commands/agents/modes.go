package agents

import (
	"fmt"
	"os"
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
		Allows: "anything but the cases and the platform's files",
		Brief: "No mode is on. Everything but the cases and the platform's files may be edited; " +
			"a Rule #11 task has not started, and a case changes only in spec mode.",
		allow: func(rel string) bool { return !feature(rel) && !platform(rel) },
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
		Name:   "redgreen",
		Allows: "the steps and production code — anything but the cases and the platform's files",
		Brief: "Rule #11 step 3: \"Red-green\" and \"Changing an approved specification\". The specification was " +
			"approved by switching to this mode and is frozen: a correction to a case is a return to spec mode. " +
			"Bind the cases and run them red, then make them green, stop.",
		allow: func(rel string) bool { return !feature(rel) && !platform(rel) },
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
// replaces one by naming it. proto follows proto_dir, so in a monorepo it lands
// on the shared contracts by itself.
func defaultAreas(protoDir string) []Area {
	return []Area{
		{Name: "gomod", Paths: []string{"go.mod", "go.sum"}, Modes: []string{"platform"}, Grant: []string{"redgreen"}},
		{Name: "steps", Paths: []string{"tests/*_test.go"}, Grant: []string{"platform"}},
		{Name: "proto", Paths: []string{strings.TrimSuffix(protoDir, "/") + "/"}, Modes: []string{"spec", "platform"}},
	}
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
func Areas(cfg command.Config) ([]Area, error) {
	byName := map[string]Area{}
	for _, a := range defaultAreas(cfg.ProtoDir) {
		byName[a.Name] = a
	}
	for name, a := range cfg.Agents.Areas {
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
	Root   string
	Areas  []Area
	claims []claim
}

// A claim is one path of an area: its names up to the first wildcard as on disk, the rest as written.
type claim struct {
	area    Area
	path    string
	literal []string
	glob    []string
	dir     bool
}

func newRules(root string, areas []Area) (Rules, error) {
	root, err := resolveExisting(root)
	if err != nil {
		return Rules{}, err
	}
	r := Rules{Root: root, Areas: areas}
	for _, a := range areas {
		for _, p := range a.Paths {
			names := strings.Split(path.Clean(p), "/")
			i := slices.IndexFunc(names, func(name string) bool { return strings.ContainsAny(name, `*?[\`) })
			if i < 0 {
				i = len(names)
			}
			base := filepath.Join(append([]string{root}, names[:i]...)...)
			resolved, err := resolveExisting(base)
			if err != nil {
				return Rules{}, err
			}
			for _, b := range slices.Compact([]string{r.written(base), resolved}) {
				r.claims = append(r.claims, claim{area: a, path: p, literal: parts(b), glob: names[i:], dir: strings.HasSuffix(p, "/")})
			}
		}
	}
	return r, nil
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
	names := parts(abs)
	var won *claim
	for i, c := range r.claims {
		if c.matches(names) && (won == nil || c.outranks(*won)) {
			won = &r.claims[i]
		}
	}
	if won == nil {
		return Area{}, false
	}
	return won.area, true
}

// A path that another area names alike, and outranks, is never answered with a.
func (r Rules) claimed(a Area) []string {
	var paths []string
	for _, c := range r.claims {
		if c.area.Name != a.Name || slices.Contains(paths, c.path) {
			continue
		}
		if !slices.ContainsFunc(r.claims, func(o claim) bool {
			return slices.Equal(o.literal, c.literal) && slices.Equal(o.glob, c.glob) && o.outranks(c)
		}) {
			paths = append(paths, c.path)
		}
	}
	return paths
}

func (c claim) matches(names []string) bool {
	n := len(c.literal) + len(c.glob)
	if len(names) < n || (!c.dir && len(names) > n) || !slices.Equal(names[:len(c.literal)], c.literal) {
		return false
	}
	for i, g := range c.glob {
		if ok, _ := path.Match(g, names[len(c.literal)+i]); !ok {
			return false
		}
	}
	return true
}

func (c claim) outranks(o claim) bool {
	if len(c.literal) != len(o.literal) {
		return len(c.literal) > len(o.literal)
	}
	if len(c.glob) != len(o.glob) {
		return len(c.glob) > len(o.glob)
	}
	return c.area.Name < o.area.Name
}

// written follows only the symlinks that lead to the root, so a path keeps the table of the service it sits in.
func (r Rules) written(p string) string {
	parent := filepath.Dir(p)
	if parent == p {
		return p
	}
	dir := r.written(parent)
	at := filepath.Join(dir, listed(dir, filepath.Base(p)))
	if _, below := within(r.Root, dir); below {
		return at
	}
	if fi, err := os.Lstat(at); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		return at
	}
	if resolved, err := resolveExisting(at); err == nil {
		if _, leads := within(resolved, r.Root); leads {
			return resolved
		}
	}
	return at
}

func parts(abs string) []string {
	return strings.Split(strings.TrimSuffix(filepath.ToSlash(abs), "/"), "/")
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
