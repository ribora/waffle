package plural

import "testing"

// The result becomes a migration's table name and a template function authors
// call, so each rule is pinned by an entity name a generator would really see.
func TestPlural(t *testing.T) {
	cases := []struct {
		name     string
		singular string
		want     string
	}{
		{"Default", "user", "users"},
		{"Underscored", "order_item", "order_items"},
		{"TrailingDigit", "user2", "user2s"},

		{"SpecialIndex", "index", "indices"},
		{"SpecialMatrix", "matrix", "matrices"},
		{"SpecialVertex", "vertex", "vertices"},

		{"SibilantS", "address", "addresses"},
		{"SibilantSh", "wish", "wishes"},
		{"SibilantTch", "match", "matches"},
		{"SibilantX", "box", "boxes"},

		{"ConsonantO", "hero", "heroes"},
		{"VowelO", "radio", "radios"},

		{"ConsonantY", "category", "categories"},
		{"VowelY", "key", "keys"},

		// "th" is not a sibilant ending; only "sh" and "tch" are listed.
		{"ThIsNotSibilant", "path", "paths"},
	}

	for _, c := range cases {
		t.Run(
			c.name,
			func(t *testing.T) {
				if got := Plural(c.singular); got != c.want {
					t.Fatalf("Plural(%q) = %q, want %q", c.singular, got, c.want)
				}
			},
		)
	}
}

// A special plural wins over the sibilant rule it also matches: "vertex" would
// otherwise come out "vertexes".
func TestPluralPrefersASpecialOverTheSibilantRule(t *testing.T) {
	if got := Plural("vertex"); got != "vertices" {
		t.Fatalf("Plural(\"vertex\") = %q, want \"vertices\"", got)
	}
}

// Entity names reach here lowercased by ValidateName, so the special table is
// never consulted for anything else — a capitalised one falls through.
func TestPluralSpecialsAreCaseSensitive(t *testing.T) {
	if got := Plural("Index"); got != "Indexes" {
		t.Fatalf("Plural(\"Index\") = %q, want \"Indexes\"", got)
	}
}

// Known wrong English. It is here so a future fix to the "o" rule is a
// deliberate change to a generated table name rather than a silent one.
func TestPluralGetsConsonantOWrongForLoanWords(t *testing.T) {
	for singular, want := range map[string]string{
		"photo": "photoes",
		"piano": "pianoes",
		"solo":  "soloes",
	} {
		if got := Plural(singular); got != want {
			t.Fatalf("Plural(%q) = %q, want %q", singular, got, want)
		}
	}
}

// The two-character guards mean a one-letter entity never reaches the "o" and
// "y" rules; ValidateName accepts one, so this input is reachable.
func TestPluralOnShortInput(t *testing.T) {
	for singular, want := range map[string]string{
		"":  "s",
		"o": "os",
		"y": "ys",
		"u": "us",
		"s": "ses",
	} {
		if got := Plural(singular); got != want {
			t.Fatalf("Plural(%q) = %q, want %q", singular, got, want)
		}
	}
}
