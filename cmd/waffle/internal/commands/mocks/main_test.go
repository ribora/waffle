package mocks

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"gitlab.com/gorib/waffle/cmd/waffle/internal/pkg/command"
)

const module = "gitlab.trgdev.com/gotrg/services/mercator"

func TestPlan(t *testing.T) {
	listed := map[string][]string{
		"gitlab.com/gorib/sql":              {"Db"},
		"google.golang.org/grpc":            {"ClientConnInterface"},
		module + "/domain/vendors/service":  {"VendorRepository", "ImageClient"},
		module + "/domain/product/service":  {"ImageClient"},
		module + "/api/hermes/proto/seller": {"SellerServiceClient"},
	}

	got := plan(listed, filepath.Join("internal", "dimock"))
	if len(got) != len(listed) {
		t.Fatalf("planned %d invocations for %d packages", len(got), len(listed))
	}

	byPath := map[string]invocation{}
	for _, inv := range got {
		byPath[inv.Path] = inv
	}

	// One destination per package, and no two the same.
	seen := map[string]bool{}
	for _, inv := range got {
		if seen[inv.Destination] {
			t.Errorf("%s reuses destination %s", inv.Path, inv.Destination)
		}
		seen[inv.Destination] = true
	}
	if want := filepath.Join("internal", "dimock", "vendors_service.go"); byPath[module+"/domain/vendors/service"].Destination != want {
		t.Errorf("destination = %s, want %s", byPath[module+"/domain/vendors/service"].Destination, want)
	}

	// ImageClient is declared by two packages, so both are renamed; nothing else is.
	vendors := byPath[module+"/domain/vendors/service"]
	product := byPath[module+"/domain/product/service"]
	if got := vendors.MockNames["ImageClient"]; got != "MockVendorsServiceImageClient" {
		t.Errorf("vendors ImageClient renamed to %q", got)
	}
	if got := product.MockNames["ImageClient"]; got != "MockProductServiceImageClient" {
		t.Errorf("product ImageClient renamed to %q", got)
	}
	if _, renamed := vendors.MockNames["VendorRepository"]; renamed {
		t.Error("VendorRepository is unique but was renamed anyway")
	}
	if byPath["gitlab.com/gorib/sql"].MockNames != nil {
		t.Error("a package with no name collision carries -mock_names")
	}

	// Interfaces are sorted, so the argv does not reshuffle between runs.
	if !slices.Equal(vendors.Interfaces, []string{"ImageClient", "VendorRepository"}) {
		t.Errorf("interfaces = %v, want them sorted", vendors.Interfaces)
	}
}

func TestInvocationArgs(t *testing.T) {
	inv := invocation{
		Path:        "gitlab.com/gorib/sql",
		Interfaces:  []string{"Db", "Tx"},
		Destination: "internal/dimock/sql.go",
	}
	want := []string{
		"tool", "mockgen", "-typed", "-package", "dimock",
		"-destination", "internal/dimock/sql.go",
		"gitlab.com/gorib/sql", "Db,Tx",
	}
	if got := inv.args("dimock"); !slices.Equal(got, want) {
		t.Errorf("args = %v\nwant %v", got, want)
	}

	inv.MockNames = map[string]string{"Db": "MockSqlDb"}
	if got := inv.args("dimock"); !slices.Contains(got, "Db=MockSqlDb") {
		t.Errorf("a renamed interface is missing from %v", got)
	}
}

func seam(field string, leaf bool, refs ...command.TypeRef) command.Seam {
	return command.Seam{Field: field, Types: refs, Leaf: leaf}
}

func TestUnlisted(t *testing.T) {
	sqlDb := command.TypeRef{Path: "gitlab.com/gorib/sql", Name: "Db"}
	conn := command.TypeRef{Path: "google.golang.org/grpc", Name: "ClientConnInterface"}
	repo := command.TypeRef{Path: module + "/domain/alert/service", Name: "BackInStockRepository"}

	seams := []command.Seam{
		seam("SqlDb", true, sqlDb),
		seam("AlertServiceBackInStockRepository", false, repo),
		// Two fields of one type collapse to a single entry.
		seam("BaseGrpcClientConnInterfaceForA", true, conn),
		seam("BaseGrpcClientConnInterfaceForB", true, conn),
		// A composite is never suggested: no single mock satisfies it.
		seam("Composite", false,
			command.TypeRef{Path: module + "/domain/x/cmd/grpc", Name: "Management"},
			command.TypeRef{Path: module + "/domain/y/adapter", Name: "Management"}),
	}
	cfg := command.Config{Mocks: command.MocksConfig{
		Generate: map[string][]string{"gitlab.com/gorib/sql": {"Db"}},
		Ignore:   map[string][]string{"google.golang.org/grpc": {"ClientConnInterface"}},
	}}

	got := unlisted(seams, cfg)
	want := []candidate{{Path: module + "/domain/alert/service", Name: "BackInStockRepository"}}
	if !slices.Equal(got, want) {
		t.Errorf("unlisted = %+v, want %+v", got, want)
	}

	if merged := composites(seams); len(merged) != 1 || merged[0].Field != "Composite" {
		t.Errorf("composites = %v", merged)
	}
}

func TestEntriesOrdersTiers(t *testing.T) {
	cands := []candidate{
		{Path: module + "/domain/alert/service", Name: "BackInStockRepository"},
		{Path: "gitlab.com/gorib/sql", Name: "Db"},
		{Path: module + "/pkg/notifier", Name: "Notifier"},
		{Path: module + "/api/hermes/proto/seller", Name: "SellerServiceClient"},
		{Path: "net", Name: "Addr"},
	}

	body := strings.Join(entries(cands, module, "  "), "\n")
	order := []string{"gitlab.com/gorib/sql", "/api/hermes", "/pkg/notifier", "/domain/alert", "net: [Addr]"}
	at := -1
	for _, want := range order {
		i := strings.Index(body, want)
		if i < 0 {
			t.Fatalf("%s is missing from:\n%s", want, body)
		}
		if i < at {
			t.Errorf("%s is out of tier order in:\n%s", want, body)
		}
		at = i
	}
	if !strings.Contains(body, "  gitlab.com/gorib/sql: [Db]") {
		t.Errorf("an entry is not rendered at the given indent:\n%s", body)
	}
}

// The line that matters most in practice holds a leaf and an interior port side
// by side — warehouse puts SecurityAdapter next to three repositories.
func TestEntriesMarksLeaves(t *testing.T) {
	service := module + "/domain/warehouse/service"
	cands := []candidate{
		{Path: "gitlab.com/gorib/sql", Name: "Db", Leaf: true},
		{Path: service, Name: "SecurityAdapter", Leaf: true},
		{Path: service, Name: "FacilityRepository"},
		{Path: service, Name: "UnitRepository"},
		{Path: "gitlab.com/gorib/grpc/server", Name: "ErrorMapper"},
	}

	body := strings.Join(entries(cands, module, "  "), "\n")
	for _, want := range []string{
		"gitlab.com/gorib/sql: [Db]  # leaf",
		service + ":\n    - FacilityRepository\n    - SecurityAdapter  # leaf\n    - UnitRepository",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q in:\n%s", want, body)
		}
	}
	for _, line := range strings.Split(body, "\n") {
		if strings.Contains(line, "ErrorMapper") && strings.Contains(line, "# leaf") {
			t.Errorf("an interior seam is marked as a leaf: %s", line)
		}
	}
}

// Found on herald: two constituents in packages that share a last segment read
// as the same entry, and a package named several times made every suffix grow to
// the full import path.
func TestSuggestionShortensCompositesUnambiguously(t *testing.T) {
	grpc := module + "/domain/notification/cmd/grpc"
	merged := []command.Seam{
		seam("NotificationAdapterTemplateManagement", false,
			command.TypeRef{Path: module + "/domain/notification/adapter", Name: "TemplateManagement"},
			command.TypeRef{Path: module + "/domain/document/adapter", Name: "TemplateManagement"}),
		seam("ScribusCommandNotificationManagement", false,
			command.TypeRef{Path: module + "/domain/notification/cmd/scribus", Name: "NotificationManagement"},
			command.TypeRef{Path: grpc, Name: "ChannelManagement"},
			command.TypeRef{Path: grpc, Name: "EventManagement"}),
	}

	body := suggestion(nil, merged, module)
	for _, want := range []string{
		"notification/adapter.TemplateManagement + document/adapter.TemplateManagement",
		"scribus.NotificationManagement + grpc.ChannelManagement + grpc.EventManagement",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q in:\n%s", want, body)
		}
	}
}

func TestFindKeys(t *testing.T) {
	cases := []struct {
		name            string
		lines           []string
		mocks, generate int
	}{
		{"neither", []string{"mapper: dto"}, -1, -1},
		{"mocks only", []string{"mocks:", "  ignore:", "    net: [Addr]"}, 0, -1},
		{"both", []string{"mocks:", "  dir: internal/dimock", "  generate:"}, 0, 2},
		{"generate under a later key does not count", []string{"mocks:", "mapper: dto", "  generate:"}, 0, -1},
		{"comments do not end the block", []string{"mocks:", "# a note", "  generate:"}, 0, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mocks, generate := findKeys(tc.lines)
			if mocks != tc.mocks || generate != tc.generate {
				t.Errorf("findKeys = (%d, %d), want (%d, %d)", mocks, generate, tc.mocks, tc.generate)
			}
		})
	}
}

func TestAddToConfig(t *testing.T) {
	missing := []candidate{{Path: "gitlab.com/gorib/sql", Name: "Db", Leaf: true}}

	t.Run("appends the whole group when absent", func(t *testing.T) {
		dir := inModule(t)
		write(t, filepath.Join(dir, "waffle.yaml"), "# conventions\nmapper: goverter\n")

		if err := addToConfig(missing, module); err != nil {
			t.Fatal(err)
		}
		body := read(t, filepath.Join(dir, "waffle.yaml"))
		if !strings.Contains(body, "# conventions") || !strings.Contains(body, "mapper: goverter") {
			t.Errorf("the existing file was not preserved:\n%s", body)
		}
		if !strings.Contains(body, "mocks:\n  generate:\n") {
			t.Errorf("the group was not added:\n%s", body)
		}
		if !strings.Contains(body, "    gitlab.com/gorib/sql: [Db]") {
			t.Errorf("the entry is not indented under generate:\n%s", body)
		}
	})

	t.Run("inserts under an existing generate", func(t *testing.T) {
		dir := inModule(t)
		write(t, filepath.Join(dir, "waffle.yaml"),
			"mocks:\n  generate:\n    gitlab.com/gorib/pry: [Logger]\n")

		if err := addToConfig(missing, module); err != nil {
			t.Fatal(err)
		}
		body := read(t, filepath.Join(dir, "waffle.yaml"))
		if strings.Count(body, "generate:") != 1 {
			t.Errorf("a second generate: key was added:\n%s", body)
		}
		if !strings.Contains(body, "gitlab.com/gorib/pry: [Logger]") {
			t.Errorf("the existing entry was lost:\n%s", body)
		}
		if !strings.Contains(body, "gitlab.com/gorib/sql: [Db]") {
			t.Errorf("the new entry is missing:\n%s", body)
		}
	})

	t.Run("adds generate to a group that only has ignore", func(t *testing.T) {
		dir := inModule(t)
		write(t, filepath.Join(dir, "waffle.yaml"),
			"mocks:\n  ignore:\n    net: [Addr]\n")

		if err := addToConfig(missing, module); err != nil {
			t.Fatal(err)
		}
		body := read(t, filepath.Join(dir, "waffle.yaml"))
		if strings.Count(body, "mocks:") != 1 {
			t.Errorf("a second mocks: key was added:\n%s", body)
		}
		if !strings.Contains(body, "  generate:") || !strings.Contains(body, "  ignore:") {
			t.Errorf("both keys should be present:\n%s", body)
		}
	})
}

// The block style is only worth anything if the loader reads back what the
// report prints, indentation and all.
func TestWrittenConfigLoadsBack(t *testing.T) {
	service := module + "/domain/warehouse/service"
	missing := []candidate{
		{Path: "gitlab.com/gorib/sql", Name: "Db", Leaf: true},
		{Path: service, Name: "SecurityAdapter", Leaf: true},
		{Path: service, Name: "FacilityRepository"},
		{Path: service, Name: "UnitRepository"},
	}

	dir := inModule(t)
	write(t, filepath.Join(dir, "waffle.yaml"), "mapper: goverter\n")
	if err := addToConfig(missing, module); err != nil {
		t.Fatal(err)
	}

	cfg, err := command.New("t", "", templates, nil).ProjectConfig(module)
	if err != nil {
		t.Fatalf("the config this command just wrote does not load: %v\n%s",
			err, read(t, filepath.Join(dir, "waffle.yaml")))
	}
	if got := cfg.Mocks.Generate["gitlab.com/gorib/sql"]; !slices.Equal(got, []string{"Db"}) {
		t.Errorf("the inline entry came back as %v", got)
	}
	want := []string{"FacilityRepository", "SecurityAdapter", "UnitRepository"}
	if got := cfg.Mocks.Generate[service]; !slices.Equal(got, want) {
		t.Errorf("the block entry came back as %v, want %v", got, want)
	}
	if cfg.Mapper != "goverter" {
		t.Errorf("an unrelated key was lost: mapper = %q", cfg.Mapper)
	}
}

func inModule(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	write(t, filepath.Join(dir, "go.mod"), "module "+module+"\n\ngo 1.25\n")
	// macOS puts TempDir under a symlink, and ModuleRoot resolves the real path.
	real, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	return real
}

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}
