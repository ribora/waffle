package scribus

import "testing"

func TestMatchTopic(t *testing.T) {
	cases := []struct {
		pat, key string
		want     bool
	}{
		{"#", "order.created", true},
		{"#", "", true},
		{"order.*", "order.created", true},
		{"order.*", "order.created.late", false},
		{"order.*", "order", false},
		{"order.#", "order", true},
		{"order.#", "order.created.late", true},
		{"*.created", "order.created", true},
		{"*.created", "created", false},
		{"order.created", "order.created", true},
		{"order.created", "order.updated", false},
		{"a.*.c", "a.b.c", true},
		{"a.*.c", "a.c", false},
		{"a.#.c", "a.x.y.c", true},
	}
	for _, c := range cases {
		if got := matchTopic(c.pat, c.key); got != c.want {
			t.Errorf("matchTopic(%q, %q) = %v, want %v", c.pat, c.key, got, c.want)
		}
	}
}

func TestOverlap(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"#", "order.created", true},
		{"order.*", "*.created", true},
		{"orders.created", "orders.*", true},
		{"order.created", "order.created", true},
		{"order.created", "order.updated", false},
		{"order.*", "payment.*", false},
		{"a.b", "a.c", false},
		{"order.#", "order.created.late", true},
		{"a.#", "a.b.c", true},
		{"a.*", "a.b.c", false},
		{"*.*", "a.b", true},
		{"*.*", "a.b.c", false},
	}
	for _, c := range cases {
		if got := overlap(c.a, c.b); got != c.want {
			t.Errorf("overlap(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
		// overlap must be symmetric
		if got := overlap(c.b, c.a); got != c.want {
			t.Errorf("overlap(%q, %q) [swapped] = %v, want %v", c.b, c.a, got, c.want)
		}
	}
}
