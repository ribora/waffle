package agents

import (
	"path/filepath"
	"strings"
	"testing"

	"gitlab.com/gorib/waffle/cmd/waffle/internal/pkg/command"
)

func config(protoDir string, areas map[string]command.AreaConfig) command.Config {
	return command.Config{ProtoDir: protoDir, Agents: command.AgentsConfig{Areas: areas}}
}

// A monorepo service at /repo/services/svc: proto_dir points at the shared
// contracts, and waffle.yaml declares the rest of shared/ as areas.
func monorepoRules(t *testing.T) Rules {
	t.Helper()
	areas, err := Areas(config("../../shared/api/proto", map[string]command.AreaConfig{
		"events": {Paths: []string{"../../shared/api/events/"}, Modes: []string{"spec", "platform"}},
		"shared": {Paths: []string{"../../shared/pkg/"}, Modes: []string{"platform"}},
		"pb":     {Paths: []string{"../../shared/api/pb/"}, Modes: []string{}},
	}))
	if err != nil {
		t.Fatal(err)
	}
	return Rules{Root: "/repo/services/svc", Areas: areas}
}

func TestModesAndAreasAllow(t *testing.T) {
	paths := []string{
		"domain/price/service/quote.go",
		"waffle.yaml",
		"tests/internal/harness/harness.go",
		"tests/feature/quote.feature",
		"tests/steps_test.go",
		"CONTRIBUTING.md",
		"go.mod",
		"domain/price/model/quote.go",
		"../../shared/api/events/price.go",
		"../../shared/pkg/money/money.go",
		"../../shared/api/pb/price/service.pb.go",
		"../../shared/api/proto/price/service.proto",
	}
	want := map[string][]bool{
		NoMode:     {true, false, false, false, true, false, false, true, false, false, false, false},
		"spec":     {false, false, false, true, false, false, false, false, true, false, false, true},
		"redgreen": {true, false, false, false, true, false, false, true, false, false, false, false},
		"platform": {true, true, true, false, false, false, true, true, true, true, false, true},
	}
	if len(want) != len(modes) {
		t.Fatalf("the table covers %d modes, the code has %d", len(want), len(modes))
	}
	r := monorepoRules(t)
	for name, allowed := range want {
		mode, err := LookupMode(name)
		if err != nil {
			t.Fatal(err)
		}
		for i, p := range paths {
			v, _ := r.Check(mode, abs(p), nil)
			if (v == Allowed) != allowed[i] {
				t.Errorf("%s mode, %s: verdict %v, want allowed=%v", name, p, v, allowed[i])
			}
		}
	}
}

func TestAGrantOpensItsAreaOnlyInItsModes(t *testing.T) {
	r := monorepoRules(t)
	cases := []struct {
		mode, grant, path string
		want              bool
	}{
		{"redgreen", "gomod", "go.mod", true},
		{"redgreen", "gomod", "go.sum", true},
		{"spec", "gomod", "go.mod", false},
		{"redgreen", "gomod", "waffle.yaml", false},
		{"redgreen", "events", "../../shared/api/events/price.go", false},
		{"platform", "steps", "tests/steps_test.go", true},
		{"platform", "steps", "tests/feature/quote.feature", false},
		{"platform", "steps", "tests/internal/fakes/fakes_test.go", true},
		{"spec", "steps", "tests/steps_test.go", false},
	}
	for _, c := range cases {
		mode, _ := LookupMode(c.mode)
		v, _ := r.Check(mode, abs(c.path), []string{c.grant})
		if (v == Allowed) != c.want {
			t.Errorf("%s mode with %s, %s: verdict %v, want allowed=%v", c.mode, c.grant, c.path, v, c.want)
		}
	}
}

func TestAPathNoAreaNamesOutsideTheServiceIsUnclaimed(t *testing.T) {
	mode, _ := LookupMode("spec")
	if v, _ := monorepoRules(t).Check(mode, "/repo/docs/readme.md", nil); v != Unclaimed {
		t.Errorf("verdict %v, want Unclaimed", v)
	}
}

func TestAnAreaClaimsItsOwnDirectory(t *testing.T) {
	mode, _ := LookupMode("spec")
	r := monorepoRules(t)
	for p, want := range map[string]Verdict{
		"../../shared/api/proto":         Allowed,
		"../../shared/api/proto/":        Allowed,
		"../../shared/api/events":        Allowed,
		"../../shared/api/protobuf/x.go": Unclaimed,
	} {
		if v, _ := r.Check(mode, abs(p), nil); v != want {
			t.Errorf("%s in spec mode: verdict %v, want %v", p, v, want)
		}
	}
}

// The refusal is what the agent acts on, so it has to name what to ask for.
func TestRefusalNamesWhatWouldAllow(t *testing.T) {
	cases := map[string]struct{ mode, path, names string }{
		"platform file with no mode":   {NoMode, "waffle.yaml", "platform mode"},
		"a case with no mode":          {NoMode, "tests/feature/quote.feature", "edited only in spec mode"},
		"production code in spec mode": {"spec", "domain/price/service/quote.go", "redgreen mode, or platform mode"},
		"a case in redgreen mode":      {"redgreen", "tests/feature/quote.feature", "spec mode"},
		"the harness in redgreen":      {"redgreen", "tests/internal/harness/harness.go", "; platform mode"},
		"go.mod in redgreen":           {"redgreen", "go.mod", "platform mode, or the gomod grant in redgreen mode"},
		"a shared event in redgreen":   {"redgreen", "../../shared/api/events/price.go", "spec mode, or platform mode"},
		"generated code":               {"platform", "../../shared/api/pb/price/service.pb.go", "no mode opens"},
	}
	r := monorepoRules(t)
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			mode, _ := LookupMode(c.mode)
			v, reason := r.Check(mode, abs(c.path), nil)
			if v != Refused || !strings.Contains(reason, c.names) {
				t.Errorf("verdict %v, reason %q does not name %q", v, reason, c.names)
			}
		})
	}
}

// Inside a single service proto_dir is api/proto, and the contract is still
// spec's to change.
func TestProtoFollowsProtoDir(t *testing.T) {
	areas, err := Areas(config("api/proto", nil))
	if err != nil {
		t.Fatal(err)
	}
	r := Rules{Root: "/svc", Areas: areas}
	for _, c := range []struct {
		mode string
		want bool
	}{{"spec", true}, {"platform", true}, {"redgreen", false}, {NoMode, false}} {
		mode, _ := LookupMode(c.mode)
		if v, _ := r.Check(mode, "/svc/api/proto/price/service.proto", nil); (v == Allowed) != c.want {
			t.Errorf("%s mode: verdict %v, want allowed=%v", c.mode, v, c.want)
		}
	}
}

func TestWaffleYamlReplacesADefaultArea(t *testing.T) {
	areas, err := Areas(config("api/proto", map[string]command.AreaConfig{
		"gomod": {Paths: []string{"go.mod", "go.sum"}, Modes: []string{"redgreen", "platform"}},
	}))
	if err != nil {
		t.Fatal(err)
	}
	mode, _ := LookupMode("redgreen")
	if v, _ := (Rules{Root: "/svc", Areas: areas}).Check(mode, "/svc/go.mod", nil); v != Allowed {
		t.Errorf("go.mod in redgreen with gomod redeclared open there: %v", v)
	}
}

func TestAreasRejectAnUnknownMode(t *testing.T) {
	_, err := Areas(config("api/proto", map[string]command.AreaConfig{
		"events": {Paths: []string{"../events/"}, Modes: []string{"review"}},
	}))
	if err == nil || !strings.Contains(err.Error(), "agents.areas.events") {
		t.Errorf("got %v", err)
	}
}

func abs(rel string) string {
	return filepath.Clean("/repo/services/svc/" + rel)
}
