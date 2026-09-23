package agents

import (
	"path/filepath"
	"strings"
	"testing"

	"gitlab.com/gorib/waffle/cmd/waffle/internal/pkg/command"
)

// A monorepo service at /repo/services/svc whose waffle.yaml declares the
// shared contracts and library as areas.
func monorepoRules(t *testing.T) Rules {
	t.Helper()
	areas, err := Areas(command.AgentsConfig{Areas: map[string]command.AreaConfig{
		"events": {Paths: []string{"../../shared/api/events/"}, Modes: []string{"spec", "platform"}},
		"shared": {Paths: []string{"../../shared/pkg/"}, Modes: []string{"platform"}},
		"pb":     {Paths: []string{"../../shared/api/pb/"}},
	}})
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
	}
	want := map[string][]bool{
		NoMode:     {true, false, false, true, true, false, false, true, false, false, false},
		"spec":     {false, false, false, true, false, false, false, false, true, false, false},
		"steps":    {false, false, false, false, true, false, false, false, false, false, false},
		"redgreen": {true, false, false, false, false, false, false, true, false, false, false},
		"platform": {true, true, true, false, false, false, true, true, true, true, false},
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
		{"steps", "model", "domain/payment/model/receipt.go", true},
		{"steps", "model", "domain/payment/service/payment.go", false},
		{"redgreen", "gomod", "go.mod", true},
		{"redgreen", "gomod", "go.sum", true},
		{"steps", "gomod", "go.mod", false},
		{"redgreen", "gomod", "waffle.yaml", false},
		{"steps", "events", "../../shared/api/events/price.go", false},
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

// The refusal is what the agent acts on, so it has to name what to ask for.
func TestRefusalNamesWhatWouldAllow(t *testing.T) {
	cases := map[string]struct{ mode, path, names string }{
		"platform file with no mode":   {NoMode, "waffle.yaml", "platform mode"},
		"production code in spec mode": {"spec", "domain/price/service/quote.go", "redgreen mode, or platform mode"},
		"a case in steps mode":         {"steps", "tests/feature/quote.feature", "spec mode"},
		"go.mod in redgreen":           {"redgreen", "go.mod", "platform mode, or the gomod grant in redgreen mode"},
		"a model in steps":             {"steps", "domain/price/model/quote.go", "; redgreen mode, or platform mode, or the model grant in steps mode"},
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

func TestWaffleYamlReplacesADefaultArea(t *testing.T) {
	areas, err := Areas(command.AgentsConfig{Areas: map[string]command.AreaConfig{
		"gomod": {Paths: []string{"go.mod", "go.sum"}, Modes: []string{"redgreen", "platform"}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	mode, _ := LookupMode("redgreen")
	if v, _ := (Rules{Root: "/svc", Areas: areas}).Check(mode, "/svc/go.mod", nil); v != Allowed {
		t.Errorf("go.mod in redgreen with gomod redeclared open there: %v", v)
	}
}

func TestAreasRejectAnUnknownMode(t *testing.T) {
	_, err := Areas(command.AgentsConfig{Areas: map[string]command.AreaConfig{
		"events": {Paths: []string{"../events/"}, Modes: []string{"review"}},
	}})
	if err == nil || !strings.Contains(err.Error(), "agents.areas.events") {
		t.Errorf("got %v", err)
	}
}

func abs(rel string) string {
	return filepath.Clean("/repo/services/svc/" + rel)
}
