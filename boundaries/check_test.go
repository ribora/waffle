package boundaries

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// A case that passed because the walk found nothing would prove nothing.
func TestPassesWhatTheAdaptersReach(t *testing.T) {
	for _, c := range []struct {
		dir     string
		opts    []Option
		reaches []string
	}{
		{dir: "adapter_to_model", reaches: []string{"domain/rent/adapter -> domain/invoice/model"}},
		{dir: "adapter_nested", reaches: []string{"domain/payment/adapter/vault -> domain/vault/model"}},
		{dir: "wire_adapter_to_service", reaches: []string{
			"domain/rent/adapter -> domain/invoice/model",
			"domain/rent/adapter -> domain/invoice/service",
		}},
		{dir: "layers_within_domain", reaches: []string{
			"domain/rent/adapter/rf/sberbank -> domain/rent/adapter/rf",
			"domain/rent/cmd/grpc -> domain/rent/cmd/grpc/convert",
		}},
		// Where the directive finds the root wherever it sits, and where there is
		// no directive and the test names it instead.
		{dir: "two_roots"},
		{dir: "root_inside_domain"},
		{dir: "root_without_diogen", opts: []Option{Roots("config/di")}},
	} {
		t.Run(c.dir, func(t *testing.T) {
			found := inspect(t, c.dir, c.opts...)

			if err := Verify(found); err != nil {
				t.Fatalf("the check refused a legal crossing: %v", err)
			}
			if got := names(found); !slices.Equal(got, c.reaches) {
				t.Errorf("crossings = %v, want %v", got, c.reaches)
			}
		})
	}
}

func TestRefusesEveryOtherDoor(t *testing.T) {
	for _, c := range []struct {
		dir  string
		says []string
	}{
		{"err_service_crosses", []string{
			"domain/rent/service -> domain/invoice/model",
			"domain/rent/service/contract.go:6",
			"outside an adapter",
		}},
		{"err_cmd_crosses", []string{
			"domain/rent/cmd/grpc -> domain/invoice/model",
			"domain/rent/cmd/grpc/service.go:6",
		}},
		{"err_adapter_to_service", []string{
			"domain/rent/adapter -> domain/invoice/service",
			"domain/rent/adapter/invoice.go:6",
		}},
		{"err_pkg_imports_domain", []string{
			"pkg/billing -> domain/invoice/model",
			"pkg/billing/total.go:3",
			"lib/report -> domain/invoice/model",
			"lib/report/report.go:3",
			"outside every domain",
			"from a composition root",
		}},
		{"err_wire_to_neighbour_adapter", []string{
			"domain/rent/adapter -> domain/invoice/adapter/pg",
			"config/di/di.go:16",
			"outside an adapter",
		}},
		{"err_service_reaches_cmd", []string{
			"domain/rent/service -> domain/rent/cmd/grpc",
			"domain/rent/service/contract.go:6",
			"closed to it",
		}},
		{"err_cmd_reaches_adapter", []string{
			"domain/rent/cmd/grpc -> domain/rent/adapter/postgres",
			"domain/rent/cmd/grpc/service.go:6",
			"cmd/ only from inside cmd/",
		}},
		{"err_service_reaches_adapter", []string{
			"domain/rent/service -> domain/rent/adapter/postgres",
			"domain/rent/service/contract.go:6",
			"how a seam is lost",
		}},
	} {
		t.Run(c.dir, func(t *testing.T) {
			err := Verify(inspect(t, c.dir))

			if err == nil {
				t.Fatal("the check passed an import that leaves the domain")
			}
			for _, want := range c.says {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("the refusal does not name %q:\n%s", want, err)
				}
			}
		})
	}
}

// The case the import half is blind to: the borrowed method is all primitives,
// so the neighbour can be named without importing anything of its.
func TestRefusesAWireThatLeavesNoImport(t *testing.T) {
	found := inspect(t, "err_wire_service_to_service")

	for _, c := range found {
		if c.Rule != Wiring {
			t.Fatalf("the fixture crosses by import too, which is not what it is for: %s at %s", c, c.Pos)
		}
	}
	err := Verify(found)
	if err == nil {
		t.Fatal("a neighbour's service was wired straight into a service")
	}
	for _, want := range []string{
		"domain/rent/service -> domain/invoice/service",
		"config/di/di.go:13",
		"registration answers",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not name %q:\n%s", want, err)
		}
	}
}

// The scaffolded spelling: di.Define hands back a handle, a second file wires
// it, and the provider's package is the handle's rather than config/di's.
func TestResolvesADefineHandle(t *testing.T) {
	found := inspect(t, "err_wire_handle_across_domains")

	var wired []Crossing
	for _, c := range found {
		if c.Rule == Wiring {
			wired = append(wired, c)
		}
	}
	// The same handle answers invoice's own handler, and that is not a crossing.
	if len(wired) != 1 {
		t.Fatalf("wiring crossings = %v, want only the one that leaves the domain", wired)
	}
	if got := wired[0].String(); got != "domain/rent/service -> domain/invoice/service" {
		t.Errorf("crossing = %s, want the handle resolved to invoice's service", got)
	}
	err := Verify(found)
	if err == nil {
		t.Fatal("a handle wired into a neighbour's service passed")
	}
	if !strings.Contains(err.Error(), "config/di/di_rent_domain.go:12") {
		t.Errorf("the refusal does not name the registration:\n%s", err)
	}
}

func TestHintsAtRootsOnlyWhereSomethingAssembles(t *testing.T) {
	const hint = "boundaries.Roots"

	// The same tree the option makes legal, refused because nothing names it.
	assembling := Verify(inspect(t, "root_without_diogen"))
	if assembling == nil {
		t.Fatal("an unnamed root passed")
	}
	if !strings.Contains(assembling.Error(), hint) {
		t.Errorf("a package that constructs was not told how to name itself:\n%s", assembling)
	}

	// pkg/ and lib/ reach a model for its types, and no root does only that.
	using := Verify(inspect(t, "err_pkg_imports_domain"))
	if using == nil {
		t.Fatal("plumbing reaching a domain passed")
	}
	if strings.Contains(using.Error(), hint) {
		t.Errorf("plumbing was offered a way to call itself a root:\n%s", using)
	}
}

// A fixture may reach for a neighbour the package it tests never ships against.
func TestSkipsTestFiles(t *testing.T) {
	for _, c := range inspect(t, "adapter_to_model") {
		if strings.Contains(c.From, "service") {
			t.Fatalf("a _test.go was read: %s at %s", c, c.Pos)
		}
	}
}

func inspect(t *testing.T, dir string, opts ...Option) []Crossing {
	t.Helper()
	found, err := Inspect(filepath.Join("testdata", "cases", dir), opts...)
	if err != nil {
		t.Fatal(err)
	}
	return found
}

func names(crossings []Crossing) []string {
	out := make([]string, 0, len(crossings))
	for _, c := range crossings {
		out = append(out, c.String())
	}
	slices.Sort(out)
	return out
}
