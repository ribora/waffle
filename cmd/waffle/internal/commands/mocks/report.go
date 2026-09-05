package mocks

import (
	"fmt"
	"sort"
	"strings"

	"gitlab.com/gorib/waffle/cmd/waffle/internal/pkg/command"
)

type tier int

const (
	tierExternal tier = iota
	tierAPI
	tierPkg
	tierDomain
	tierStdlib
)

var tierOrder = []tier{tierExternal, tierAPI, tierPkg, tierDomain, tierStdlib}

var tierHeading = map[tier]string{
	tierExternal: "outside this module",
	tierAPI:      "api/ — generated clients of other services",
	tierPkg:      "pkg/ — this service's own infrastructure",
	tierDomain:   "ports of this service",
	tierStdlib:   "standard library",
}

// Tiers order the report and nothing else. Whether a test uses a mock or the
// real dependency is decided per test, so nothing here is demoted.
func tierOf(path, module string) tier {
	rel, inside := strings.CutPrefix(path, module+"/")
	first, _, _ := strings.Cut(path, "/")
	switch {
	// A standard library import path has no dot in its first element, which is
	// the rule the go tool goes by too.
	case !inside && !strings.Contains(first, "."):
		return tierStdlib
	case !inside:
		return tierExternal
	case strings.HasPrefix(rel, "api/"):
		return tierAPI
	case strings.HasPrefix(rel, "pkg/"):
		return tierPkg
	default:
		return tierDomain
	}
}

// candidate is one interface a container exposes, with the one structural fact
// worth carrying: whether its default constructor takes anything else out of the
// container.
type candidate struct {
	Path string
	Name string
	Leaf bool
}

// unlisted are the interfaces a container exposes that waffle.yaml mentions in
// neither mocks.generate nor mocks.ignore. Composite seams are left out of this
// list because no single mock satisfies interface{A; B} — but they are not
// noise: they are reported separately, since a composite names the contracts one
// management object owes.
func unlisted(seams []command.Seam, cfg command.Config) []candidate {
	var out []candidate
	seen := map[string]bool{}
	for _, seam := range seams {
		if seam.Composite() {
			continue
		}
		for _, ref := range seam.Types {
			if seen[ref.String()] || cfg.Mocks.Listed(ref.Path, ref.Name) {
				continue
			}
			seen[ref.String()] = true
			out = append(out, candidate{Path: ref.Path, Name: ref.Name, Leaf: seam.Leaf})
		}
	}
	return out
}

func composites(seams []command.Seam) []command.Seam {
	var out []command.Seam
	for _, seam := range seams {
		if seam.Composite() {
			out = append(out, seam)
		}
	}
	return out
}

func packages(cands []candidate) int {
	paths := map[string]bool{}
	for _, c := range cands {
		paths[c.Path] = true
	}
	return len(paths)
}

// entries renders the mapping lines only, at the given indent, so the same text
// serves the printed report, the --check failure and the insert into waffle.yaml.
func entries(cands []candidate, module, indent string) []string {
	byPath := map[string][]candidate{}
	byTier := map[tier][]string{}
	for _, c := range cands {
		if _, known := byPath[c.Path]; !known {
			byTier[tierOf(c.Path, module)] = append(byTier[tierOf(c.Path, module)], c.Path)
		}
		byPath[c.Path] = append(byPath[c.Path], c)
	}

	var out []string
	for _, t := range tierOrder {
		paths := byTier[t]
		if len(paths) == 0 {
			continue
		}
		sort.Strings(paths)
		if len(out) > 0 {
			out = append(out, "")
		}
		out = append(out, indent+"# "+tierHeading[t])
		for _, path := range paths {
			group := byPath[path]
			sort.Slice(group, func(i, j int) bool { return group[i].Name < group[j].Name })

			// One interface stays inline — there is nothing to tell apart, and a
			// block would double the height of a report that is mostly singles.
			if len(group) == 1 {
				out = append(out, indent+fmt.Sprintf("%s: [%s]%s", path, group[0].Name, leafNote(group[0].Leaf)))
				continue
			}
			// Several go one per line, so the leaf marker sits on the interface it
			// describes rather than naming it from the end of a shared line.
			out = append(out, indent+path+":")
			for _, c := range group {
				out = append(out, indent+"  - "+c.Name+leafNote(c.Leaf))
			}
		}
	}
	return out
}

func leafNote(leaf bool) string {
	if leaf {
		return "  # leaf"
	}
	return ""
}

func suggestion(cands []candidate, merged []command.Seam, module string) string {
	var b strings.Builder
	b.WriteString("mocks:\n  generate:\n")
	for _, line := range entries(cands, module, "    ") {
		b.WriteString(line + "\n")
	}

	b.WriteString("\n# leaf — the default constructor takes nothing else out of the container, so it\n")
	b.WriteString("# is a boundary of the process. A component test replaces leaves and lets the\n")
	b.WriteString("# rest run for real; the interior entries are for unit tests, where you build\n")
	b.WriteString("# the object under test yourself and Replacements plays no part.\n")

	if len(merged) > 0 {
		b.WriteString("\n# Composite seams — one management object serving several declared\n")
		b.WriteString("# contracts. This is the list of specifications the domain owes: each\n")
		b.WriteString("# constituent is a caller, and a caller is a place a scenario binds.\n")
		b.WriteString("# No single mock satisfies interface{A; B}, so mock a constituent for a\n")
		b.WriteString("# handler unit test; the composite itself is read, not replaced.\n")
		for _, seam := range merged {
			// Shortened for reading, but only as far as it stays unambiguous:
			// two constituents can sit in packages that share a last segment.
			paths := make([]string, 0, len(seam.Types))
			for _, ref := range seam.Types {
				paths = append(paths, ref.Path)
			}
			short := distinctSuffixes(paths, "/")

			names := make([]string, 0, len(seam.Types))
			for _, ref := range seam.Types {
				names = append(names, short[ref.Path]+"."+ref.Name)
			}
			_, _ = fmt.Fprintf(&b, "#   %s — %s\n", seam.Field, strings.Join(names, " + "))
		}
	}
	return b.String()
}
