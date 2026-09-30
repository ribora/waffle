package surface

import (
	"fmt"
	"slices"
	"sort"
	"strings"
)

type Report struct {
	Unconsumed  []string `yaml:"unconsumed,omitempty"`
	Unpublished []string `yaml:"unpublished,omitempty"`
	Unserved    []string `yaml:"unserved,omitempty"`
	Uncalled    []string `yaml:"uncalled,omitempty"`
	Ambiguous   []string `yaml:"ambiguous,omitempty"`

	PatternOnly []string `yaml:"pattern_only,omitempty"`
	Loops       []string `yaml:"loops,omitempty"`
	Outside     []string `yaml:"outside,omitempty"`
}

func (r Report) Failed() bool {
	return len(r.Unconsumed)+len(r.Unpublished)+len(r.Unserved)+len(r.Uncalled)+len(r.Ambiguous) > 0
}

func (r Report) String() string {
	var b strings.Builder
	section := func(title string, items []string) {
		if len(items) == 0 {
			return
		}
		fmt.Fprintf(&b, "%s:\n", title)
		for _, i := range items {
			fmt.Fprintf(&b, "  %s\n", i)
		}
	}
	section("published, no subscriber", r.Unconsumed)
	section("subscribed, nobody publishes", r.Unpublished)
	section("called, nobody serves", r.Unserved)
	section("served, nobody calls", r.Uncalled)
	section("requested, routed by more than one service", r.Ambiguous)
	section("taken only by a wildcard subscription", r.PatternOnly)
	section("consumed by the service that publishes it", r.Loops)
	section("requested outside the set", r.Outside)
	return b.String()
}

func Verify(set []Surface) (Report, error) {
	seen := map[string]bool{}
	for _, s := range set {
		if seen[s.Service] {
			return Report{}, fmt.Errorf("service %s appears twice in the set", s.Service)
		}
		seen[s.Service] = true
	}

	type party = map[string][]string
	publishers, consumers, servers, callers := party{}, party{}, party{}, party{}
	for _, s := range set {
		for _, k := range s.Publishes {
			publishers[k] = append(publishers[k], s.Service)
		}
		for _, k := range s.Consumes {
			consumers[k] = append(consumers[k], s.Service)
		}
		for _, m := range s.Serves {
			servers[m] = append(servers[m], s.Service)
		}
		for _, m := range s.Calls {
			callers[m] = append(callers[m], s.Service)
		}
	}

	var r Report
	for _, key := range sorted(publishers) {
		var exact, wild []string
		for _, sub := range sorted(consumers) {
			switch {
			case sub == key:
				exact = append(exact, consumers[sub]...)
			case matches(sub, key):
				wild = append(wild, consumers[sub]...)
			}
		}
		from := strings.Join(publishers[key], ", ")
		switch {
		case len(exact) == 0 && len(wild) == 0:
			r.Unconsumed = append(r.Unconsumed, fmt.Sprintf("%s (from %s)", key, from))
		case len(exact) == 0:
			r.PatternOnly = append(r.PatternOnly, fmt.Sprintf("%s (from %s, taken by %s)", key, from, strings.Join(wild, ", ")))
		}
	}
	for _, sub := range sorted(consumers) {
		if wildcard(sub) || publishers[sub] != nil {
			continue
		}
		r.Unpublished = append(r.Unpublished, fmt.Sprintf("%s (by %s)", sub, strings.Join(consumers[sub], ", ")))
	}
	for _, m := range sorted(callers) {
		by := strings.Join(callers[m], ", ")
		if !isHTTP(m) {
			if servers[m] == nil {
				r.Unserved = append(r.Unserved, fmt.Sprintf("%s (by %s)", m, by))
			}
			continue
		}
		var on []string
		for _, route := range sorted(servers) {
			if isHTTP(route) && known(m) && names(route) && routeMatches(route, m) {
				on = append(on, servers[route]...)
			}
		}
		slices.Sort(on)
		on = slices.Compact(on)
		switch {
		case len(on) == 0:
			r.Outside = append(r.Outside, fmt.Sprintf("%s (by %s)", m, by))
		case len(on) > 1:
			r.Ambiguous = append(r.Ambiguous, fmt.Sprintf("%s (by %s, routed by %s)", m, by, strings.Join(on, ", ")))
		}
	}
	// A route nobody in the set requests is a way in from outside, so only RPCs
	// are held to having a caller.
	for _, m := range sorted(servers) {
		if !isHTTP(m) && callers[m] == nil {
			r.Uncalled = append(r.Uncalled, fmt.Sprintf("%s (on %s)", m, strings.Join(servers[m], ", ")))
		}
	}
	for _, s := range set {
		for _, k := range s.Publishes {
			if slices.Contains(s.Consumes, k) {
				r.Loops = append(r.Loops, fmt.Sprintf("%s: %s", s.Service, k))
			}
		}
	}
	return r, nil
}

// Receivers names the services in the set that take a message: an event by its
// key — exactly, or only through a wildcard subscription; an RPC by its full
// name; an HTTP request by the routes that name it. Kind is event, grpc or http,
// as tests/flows.yaml writes it.
func Receivers(set []Surface, kind, name string) (exact, wild []string) {
	for _, s := range set {
		switch kind {
		case "event":
			for _, sub := range s.Consumes {
				switch {
				case sub == name:
					exact = append(exact, s.Service)
				case wildcard(sub) && matches(sub, name):
					wild = append(wild, s.Service)
				}
			}
		case "grpc":
			if slices.Contains(s.Serves, name) {
				exact = append(exact, s.Service)
			}
		case "http":
			for _, route := range s.Serves {
				if isHTTP(route) && known(name) && names(route) && routeMatches(route, name) {
					exact = append(exact, s.Service)
				}
			}
		}
	}
	slices.Sort(exact)
	slices.Sort(wild)
	return slices.Compact(exact), slices.Compact(slices.DeleteFunc(wild, func(w string) bool { return slices.Contains(exact, w) }))
}

// Senders names the services in the set whose code sends a message to kind and
// name, the way Receivers reads them; for HTTP, name is the route.
func Senders(set []Surface, kind, name string) []string {
	var out []string
	for _, s := range set {
		switch kind {
		case "event":
			if slices.Contains(s.Publishes, name) {
				out = append(out, s.Service)
			}
		case "grpc":
			if slices.Contains(s.Calls, name) {
				out = append(out, s.Service)
			}
		case "http":
			for _, request := range s.Calls {
				if isHTTP(request) && known(request) && routeMatches(name, request) {
					out = append(out, s.Service)
				}
			}
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

func wildcard(sub string) bool {
	for _, w := range strings.Split(sub, ".") {
		if w == "*" || w == "#" {
			return true
		}
	}
	return false
}

// The AMQP topic rule.
func matches(pattern, key string) bool {
	return topic(strings.Split(pattern, "."), strings.Split(key, "."))
}

func topic(p, k []string) bool {
	switch {
	case len(p) == 0:
		return len(k) == 0
	case p[0] == "#":
		for i := 0; i <= len(k); i++ {
			if topic(p[1:], k[i:]) {
				return true
			}
		}
		return false
	case len(k) == 0:
		return false
	case p[0] == "*" || p[0] == k[0]:
		return topic(p[1:], k[1:])
	}
	return false
}

func sorted(m map[string][]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
