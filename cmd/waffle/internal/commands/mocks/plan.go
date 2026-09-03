package mocks

import (
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"unicode"
)

// One mockgen run: it writes a single destination per invocation, so the unit is
// the import path rather than the interface.
type invocation struct {
	Path        string
	Interfaces  []string
	Destination string
	MockNames   map[string]string // interface -> mock type, only where names collide
}

func (i invocation) args(pkg string) []string {
	out := []string{"tool", "mockgen", "-typed", "-package", pkg, "-destination", i.Destination}
	if len(i.MockNames) > 0 {
		pairs := make([]string, 0, len(i.MockNames))
		for _, name := range i.Interfaces {
			if mock, renamed := i.MockNames[name]; renamed {
				pairs = append(pairs, name+"="+mock)
			}
		}
		out = append(out, "-mock_names", strings.Join(pairs, ","))
	}
	return append(out, i.Path, strings.Join(i.Interfaces, ","))
}

// plan turns the mocks: map into mockgen invocations. Every destination lands in
// one flat package, so two interfaces of the same name from different packages
// would collide — those, and only those, are renamed.
func plan(listed map[string][]string, dir string) []invocation {
	paths := make([]string, 0, len(listed))
	for path := range listed {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	files := distinctSuffixes(paths, "_")
	renames := mockNames(listed, paths)

	out := make([]invocation, 0, len(paths))
	for _, path := range paths {
		names := append([]string(nil), listed[path]...)
		sort.Strings(names)

		inv := invocation{
			Path:        path,
			Interfaces:  names,
			Destination: filepath.Join(dir, fileName(files[path])+".go"),
		}
		for _, name := range names {
			if mock, renamed := renames[path+"."+name]; renamed {
				if inv.MockNames == nil {
					inv.MockNames = map[string]string{}
				}
				inv.MockNames[name] = mock
			}
		}
		out = append(out, inv)
	}
	return out
}

// An interface named by two packages needs the shortest path suffix that tells
// them apart — unconditional prefixing would turn payment.PaymentServiceClient
// into MockPaymentPaymentServiceClient.
func mockNames(listed map[string][]string, paths []string) map[string]string {
	owners := map[string][]string{}
	for _, path := range paths {
		for _, name := range listed[path] {
			owners[name] = append(owners[name], path)
		}
	}

	out := map[string]string{}
	for name, clashing := range owners {
		if len(clashing) < 2 {
			continue
		}
		for path, suffix := range distinctSuffixes(clashing, "_") {
			out[path+"."+name] = "Mock" + camel(suffix) + name
		}
	}
	return out
}

// The last segment of each path, lengthened one segment at a time until no two
// are the same. Repeats are collapsed first: one composite seam names the same
// package several times, and no suffix can ever tell a path from itself.
func distinctSuffixes(paths []string, sep string) map[string]string {
	unique := make([]string, 0, len(paths))
	for _, p := range paths {
		if !slices.Contains(unique, p) {
			unique = append(unique, p)
		}
	}

	out := make(map[string]string, len(unique))
	for k := 1; k <= 16; k++ {
		clear(out)
		seen := make(map[string]bool, len(unique))
		clash := false
		for _, p := range unique {
			segments := strings.Split(p, "/")
			if k < len(segments) {
				segments = segments[len(segments)-k:]
			}
			joined := strings.Join(segments, sep)
			if seen[joined] {
				clash = true
				break
			}
			seen[joined] = true
			out[p] = joined
		}
		if !clash {
			break
		}
	}
	return out
}

func fileName(suffix string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			return unicode.ToLower(r)
		}
		return '_'
	}, suffix)
}

func camel(suffix string) string {
	var out strings.Builder
	upper := true
	for _, r := range suffix {
		switch {
		case !unicode.IsLetter(r) && !unicode.IsDigit(r):
			upper = true
		case upper:
			out.WriteRune(unicode.ToUpper(r))
			upper = false
		default:
			out.WriteRune(r)
		}
	}
	return out.String()
}
