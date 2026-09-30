package tests

import (
	"testing"

	"gitlab.com/gorib/waffle/surface"
)

// What this service publishes, consumes, serves and calls is checked against
// the others' records by the project; this keeps its own record honest.
func TestSurfaceMatchesTheCode(t *testing.T) {
	if err := surface.Check(); err != nil {
		t.Fatal(err)
	}
}
