package {{.Package}}_test

import (
	"testing"

	"gitlab.com/gorib/waffle/replacements"
)

// A seam the list does not grant is a question for whoever owns waffle.yaml —
// granting it yourself is answering it yourself.
// One copy answers for the whole module: the check walks up to go.mod.
func TestReplacementsStayInsideTheList(t *testing.T) {
	if err := replacements.Check(); err != nil {
		t.Fatal(err)
	}
}
