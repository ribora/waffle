package flows

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"gitlab.com/gorib/waffle/tools/trace"
)

func projectOf(steps ...step) project {
	p := project{services: map[string]service{}, steps: map[string]step{}, consumes: map[string][]string{}, produces: map[string][]string{}}
	for _, s := range steps {
		p.services[s.service] = service{}
		p.steps[s.id()] = s
		p.consumes[s.input.Message] = append(p.consumes[s.input.Message], s.id())
		for _, b := range s.input.Branches {
			for _, m := range b.Sends {
				p.produces[m] = append(p.produces[m], s.service)
			}
		}
	}
	return p
}

func in(service, message string, branches ...Branch) step {
	return step{service: service, input: Input{Message: message, Branches: branches}}
}

// The gateway charges only when the cart is not free; what the charge sends on
// happens only then, so it belongs inside the block, not after it.
func TestWhatAStepSendsSitsUnderTheConditionThatReachedIt(t *testing.T) {
	p := projectOf(
		in("gateway", "http POST /pay",
			Branch{Sends: []string{"grpc /pay.Pay/Charge"}, Cases: []string{"a", "b"}},
			Branch{Cases: []string{"c"}}),
		in("pay", "grpc /pay.Pay/Charge",
			Branch{Sends: []string{"event paid"}, Cases: []string{"d"}}),
		in("books", "event paid", Branch{Cases: []string{"e"}}),
	)
	order, _ := p.walk("gateway: http POST /pay")
	body := string(page(p, Flow{Id: "gateway-http-post-pay", Entry: "gateway: http POST /pay", Steps: order}, true))

	charge := strings.Index(body, "svc_gateway->>svc_pay")
	paid := strings.Index(body, "svc_pay-)svc_books")
	end := strings.Index(body, "end\n")
	if charge < 0 || paid < 0 || end < 0 {
		t.Fatalf("an arrow or the block is missing:\n%s", body)
	}
	if charge >= paid || paid >= end {
		t.Errorf("what the charge sends is outside the block that made the charge:\n%s", body)
	}
}

func TestAStepFurtherOnLinksToItsOwnPageInsteadOfRepeatingItsCases(t *testing.T) {
	p := projectOf(
		in("gateway", "http POST /pay", Branch{Sends: []string{"grpc /pay.Pay/Charge"}, Cases: []string{"x.feature: Paying"}}),
		in("pay", "grpc /pay.Pay/Charge",
			Branch{Cases: []string{"pay.feature: A charge goes through"}},
			Branch{Refusal: "NotFound method_not_found", Cases: []string{"pay.feature: An unknown method"}}),
	)
	order, _ := p.walk("gateway: http POST /pay")
	body := string(page(p, Flow{Id: "gateway-http-post-pay", Entry: "gateway: http POST /pay", Steps: order}, true))

	for _, want := range []string{
		"- nothing further — 1 case",
		"- refused: NotFound method_not_found — 1 case",
		"Its cases: [pay-grpc-pay-charge](pay-grpc-pay-charge.md)",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the page lacks %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "A charge goes through") {
		t.Errorf("the page repeats the cases of a step further on:\n%s", body)
	}
}

func TestAStepManySendReachListsThemFolded(t *testing.T) {
	var steps []step
	for _, route := range []string{"/a", "/b", "/c", "/d"} {
		steps = append(steps, in("gateway", "http GET "+route, Branch{Sends: []string{"grpc /auth.Auth/Summon"}, Cases: []string{"x"}}))
	}
	p := projectOf(append(steps, in("auth", "grpc /auth.Auth/Summon", Branch{Cases: []string{"y"}}))...)
	body := string(page(p, Flow{Id: "auth-grpc-auth-summon", Entry: "auth: grpc /auth.Auth/Summon", Steps: []string{"auth: grpc /auth.Auth/Summon"}}, false))

	for _, want := range []string{
		"— received from 4 steps\n",
		"<summary>Received from (4)</summary>",
		"- [gateway: http GET /d](gateway-http-get-d.md)\n",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the page lacks %q:\n%s", want, body)
		}
	}
}

func TestALoopIsOnePassAndAReturnIsAStep(t *testing.T) {
	for _, c := range []struct{ sent, want []string }{
		{[]string{"a", "a", "a"}, []string{"a"}},
		{[]string{"a", "b", "a", "b"}, []string{"a", "b"}},
		{[]string{"otp", "render", "store", "otp"}, []string{"otp", "render", "store", "otp"}},
		{[]string{"x", "a", "b", "a", "b", "y"}, []string{"x", "a", "b", "y"}},
	} {
		if got := passes(c.sent); !slices.Equal(got, c.want) {
			t.Errorf("passes(%v) = %v, want %v", c.sent, got, c.want)
		}
	}
}

// A link asked for, refused, the document drawn and stored, the link asked for
// again: the second ask is an arrow of its own, after the store.
func TestASendRepeatedAfterOthersIsDrawnWhereItHappened(t *testing.T) {
	p := projectOf(
		in("herald", "grpc /doc.Docs/Link", Branch{
			Sends: []string{"grpc /media.Media/Link", "grpc /media.Media/Store", "grpc /media.Media/Link"},
			Cases: []string{"a"},
		}),
		in("media", "grpc /media.Media/Link", Branch{Cases: []string{"b"}}),
		in("media", "grpc /media.Media/Store", Branch{Cases: []string{"c"}}),
	)
	order, _ := p.walk("herald: grpc /doc.Docs/Link")
	body := string(page(p, Flow{Id: "herald-grpc-docs-link", Entry: "herald: grpc /doc.Docs/Link", Steps: order}, true))

	first := strings.Index(body, "Media.Link")
	store := strings.Index(body, "Media.Store")
	again := strings.LastIndex(body, "Media.Link")
	if first >= store || store >= again {
		t.Errorf("the second link is not drawn after the store:\n%s", body)
	}
}

func TestAnEventBackToItsOwnSenderIsMarked(t *testing.T) {
	p := projectOf(
		in("rent", "command renew", Branch{Sends: []string{"event paid"}, Cases: []string{"a"}}),
		in("rent", "event paid", Branch{Cases: []string{"b"}}),
	)
	order, _ := p.walk("rent: command renew")
	body := string(page(p, Flow{Id: "rent-command-renew", Entry: "rent: command renew", Steps: order}, true))
	if !strings.Contains(body, "Note over svc_rent: its own event, back through the bus") {
		t.Errorf("the loop is not marked:\n%s", body)
	}
}

// Two files may hold a case of one title; the file the suite ran it from is the
// one its label names, whichever of them sorts last.
func TestACaseIsLabelledByTheFileItRanFrom(t *testing.T) {
	dir := t.TempDir()
	for name, body := range map[string]string{
		"contract.feature": "Feature: Contracts\n\n  Scenario: An admin without the permission is forbidden\n    When it is sent\n",
		"price.feature":    "Feature: Prices\n\n  Scenario: An admin without the permission is forbidden\n    When it is sent\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	records := []trace.Record{{
		Seq: 1, Scenario: "TestFeatures/An_admin_without_the_permission_is_forbidden",
		Feature: filepath.Join(dir, "contract.feature"), Dir: "in", Kind: "http", Name: "DELETE /contracts/{id}", Error: "HTTP 403 forbidden",
	}}

	doc := build("gateway", records, readFeatures(dir))

	got := doc.Inputs[0].Branches[0].Cases
	if len(got) != 1 || got[0] != "contract.feature: An admin without the permission is forbidden" {
		t.Errorf("cases %v, want the one of contract.feature", got)
	}
}

func TestRefusalsOfDifferentKindsAreDifferentBranches(t *testing.T) {
	run := func(scenario, refusal string, seq int64) trace.Record {
		return trace.Record{Seq: seq, Scenario: scenario, Dir: "in", Kind: "grpc", Name: "/contract.Contracts/Close", Error: refusal}
	}
	doc := build("dungeon", []trace.Record{
		run("TestFeatures/Closing_an_unknown_contract", "NotFound contract_not_found", 1),
		run("TestFeatures/Closing_a_closed_contract", "FailedPrecondition contract_is_closed", 2),
	}, features{})

	if got := len(doc.Inputs[0].Branches); got != 2 {
		t.Errorf("%d branches, want one per refusal: %+v", got, doc.Inputs[0].Branches)
	}
}
