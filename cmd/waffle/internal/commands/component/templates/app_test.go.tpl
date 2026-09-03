package {{.Package}}_test

import (
	"testing"

	"{{.Import}}"
)

// A component test boots the real container and replaces only the boundaries it
// cares about — everything else is the production wiring. To see what can go
// into Replacements:
//
//	./waffle make:tests:mocks --suggest
func TestApp(t *testing.T) {
	t.Skip("remove once the environment below is filled in")

	// The container reads the environment while it wires, so whatever the
	// registrations require has to be set first. InitAppWith returns an error
	// naming the first variable it cannot find.
	t.Setenv("ENVIRONMENT", "test")

	app, err := {{.Package}}.InitAppWith({{.Package}}.Replacements{
		// An empty Replacements builds everything for real. Name the fakes this
		// test needs and leave the rest zero.
	})
	if err != nil {
		t.Fatal(err)
	}
	// Shutdown hands back the closer instead of running it, hence the second call.
	t.Cleanup(func() {
		if err := app.Shutdown()(); err != nil {
			t.Errorf("shutdown: %v", err)
		}
	})
}
