package tests

import (
"context"
"fmt"
"os"
"testing"

"{{.Module}}/tests/internal/harness"
)

// Down once per process: whatever is up is shared by every test in the package,
// and dropping it between two of them would leave the second without a schema.
func TestMain(m *testing.M) {
harness.Env()

code := m.Run()

if err := harness.Down(context.Background()); err != nil {
// Not fatal: the run is over, and its result is what the caller wants.
_, _ = fmt.Fprintln(os.Stderr, err)
}
os.Exit(code)
}
