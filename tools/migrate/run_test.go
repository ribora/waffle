package migrate

import (
	"context"
	"strings"
	"testing"
)

// A run that never calls SetDialect accepts any driver and then builds goose's
// bookkeeping table with postgres syntax, whatever the service is connected to.
// Rejecting an unknown dialect is how that call is visible from out here.
func TestRunSetsTheDialect(t *testing.T) {
	err := run(context.Background(), nil, nil, "nonsense", "up", nil)
	if err == nil {
		t.Fatal("run accepted an unknown dialect")
	}
	if !strings.Contains(err.Error(), "nonsense") {
		t.Fatalf("the error does not name the dialect: %v", err)
	}
}
