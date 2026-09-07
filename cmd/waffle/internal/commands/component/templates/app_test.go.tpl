package {{.Package}}_test

import (
	"testing"

	"{{.Import}}"
)

// Does the container wire at all, with no infrastructure behind it. This is not
// the scenario suite's smoke test: that one lives in tests/, brings a database
// up and serves the handlers. Keep both — this one fails faster and closer to
// the wiring.
//
// What may go into Replacements is replacements.allow in waffle.yaml. To see
// everything the container exposes, granted or not:
//
//	./waffle tests:replacements
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
