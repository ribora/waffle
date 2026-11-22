package diogen_test

import (
	"testing"

	"github.com/ribora/waffle/boundaries"
)

// What this refuses is a design question, not a test to delete.
// One copy answers for the whole module: the check walks up to go.mod.
func TestDomainsStayBehindTheirAdapters(t *testing.T) {
	if err := boundaries.Check(); err != nil {
		t.Fatal(err)
	}
}
