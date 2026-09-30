package flows

import (
	"reflect"
	"testing"

	"gitlab.com/gorib/waffle/surface"
)

func traced(services ...string) project {
	p := project{services: map[string]service{}}
	for _, s := range services {
		p.services[s] = service{}
	}
	return p
}

func TestBeyondTellsWhereAnUnfollowedMessageGoes(t *testing.T) {
	p := traced("dungeon", "warehouse")
	p.surfaces = []surface.Surface{
		{Service: "dungeon", Publishes: []string{"contracts.moved"}, Calls: []string{"/warehouse.Units/Get"}},
		{Service: "warehouse", Serves: []string{"/warehouse.Units/Get"}},
		{Service: "herald", Consumes: []string{"#"}},
	}
	for m, want := range map[string]string{
		"grpc /warehouse.Units/Get": "received by warehouse, no case took it",
		"event contracts.moved":     "only a wildcard subscription: herald",
		"grpc /nobody.Serves/This":  "out of the system",
	} {
		if got := p.beyond("dungeon", m); got != want {
			t.Errorf("beyond(%s) = %q, want %q", m, got, want)
		}
	}
	if got := traced().beyond("dungeon", "event contracts.moved"); got != "not traced" {
		t.Errorf("without records: %q, want not traced", got)
	}
}

// A service without a trace cannot have followed anything, so only links between
// two traced services are held against the cases.
func TestUntestedIsWhatTheCodeLinksAndNoCaseFollows(t *testing.T) {
	p := traced("dungeon", "herald", "warehouse")
	p.surfaces = []surface.Surface{
		{Service: "apostle", Calls: []string{"/contract.Contracts/Get"}},
		{Service: "dungeon", Publishes: []string{"invoices.paid"}, Serves: []string{"/contract.Contracts/Get"}, Calls: []string{"/warehouse.Units/Get"}},
		{Service: "herald", Consumes: []string{"invoices.paid"}, Calls: []string{"/contract.Contracts/Get", "POST /api/render/text"}},
		{Service: "warehouse", Serves: []string{"/warehouse.Units/Get"}},
	}
	followed := []Link{{From: "dungeon", To: "herald", Message: "event invoices.paid"}}
	want := []string{
		"dungeon → warehouse: grpc /warehouse.Units/Get",
		"herald → dungeon: grpc /contract.Contracts/Get",
	}
	if got := p.untested(followed); !reflect.DeepEqual(got, want) {
		t.Errorf("got  %v\nwant %v", got, want)
	}
}

func TestSendersLeaveOutTheServiceItself(t *testing.T) {
	p := traced("dungeon")
	p.surfaces = []surface.Surface{
		{Service: "apostle", Calls: []string{"/checkout.Checkout/Pay"}},
		{Service: "dungeon", Serves: []string{"/checkout.Checkout/Pay"}, Calls: []string{"/checkout.Checkout/Pay"}},
	}
	s := step{service: "dungeon", input: Input{Message: "grpc /checkout.Checkout/Pay"}}
	if got := p.senders(s); !reflect.DeepEqual(got, []string{"apostle"}) {
		t.Errorf("got %v, want [apostle]", got)
	}
}
