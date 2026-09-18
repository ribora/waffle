package boundaries

import (
	"path/filepath"
	"strings"
	"testing"
)

// Nothing handed in: a service calls Check from somewhere under the module root.
func TestCheckFindsTheModuleAroundIt(t *testing.T) {
	t.Run("clean", func(t *testing.T) {
		chdir(t, "adapter_to_model", "domain", "rent", "adapter")

		if err := Check(); err != nil {
			t.Fatalf("the check refused a clean module: %v", err)
		}
	})

	t.Run("crossed", func(t *testing.T) {
		chdir(t, "err_service_crosses", "domain", "rent", "service")

		err := Check()
		if err == nil {
			t.Fatal("the check passed a module that crosses a boundary")
		}
		if !strings.Contains(err.Error(), "domain/rent/service -> domain/invoice/model") {
			t.Errorf("the refusal does not name the crossing:\n%s", err)
		}
	})
}

func chdir(t *testing.T, dir string, under ...string) {
	t.Helper()
	parts := append([]string{"testdata", "cases", dir}, under...)
	abs, err := filepath.Abs(filepath.Join(parts...))
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(abs)
}
