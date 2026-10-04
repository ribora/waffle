package surface

import (
	"reflect"
	"testing"
)

func TestVerifyClosesAPairedSet(t *testing.T) {
	r, err := Verify([]Surface{
		{Service: "orders", Publishes: []string{"orders.placed"}, Calls: []string{"/stock.S/Reserve"}},
		{Service: "stock", Consumes: []string{"orders.placed"}, Serves: []string{"/stock.S/Reserve"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if r.Failed() || r.String() != "" {
		t.Errorf("want a closed set, got:\n%s", r)
	}
}

func TestVerifyFindsWhatHasNoPartner(t *testing.T) {
	r, err := Verify([]Surface{
		{Service: "orders", Publishes: []string{"orders.placed"}, Consumes: []string{"stock.reserved"}, Calls: []string{"/stock.S/Reserve"}},
		{Service: "stock", Serves: []string{"/stock.S/Release"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := Report{
		Unconsumed:  []string{"orders.placed (from orders)"},
		Unpublished: []string{"stock.reserved (by orders)"},
		Unserved:    []string{"/stock.S/Reserve (by orders)"},
		Uncalled:    []string{"/stock.S/Release (on stock)"},
	}
	if !reflect.DeepEqual(r, want) {
		t.Errorf("got  %+v\nwant %+v", r, want)
	}
}

func TestVerifyTellsLoopsAndWildcardsWithoutFailing(t *testing.T) {
	r, err := Verify([]Surface{
		{Service: "dungeon", Publishes: []string{"invoices.paid", "contracts.moved"}, Consumes: []string{"invoices.paid"}},
		{Service: "herald", Consumes: []string{"#"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := Report{
		PatternOnly: []string{"contracts.moved (from dungeon, taken by herald)"},
		Loops:       []string{"dungeon: invoices.paid"},
	}
	if r.Failed() || !reflect.DeepEqual(r, want) {
		t.Errorf("got  %+v\nwant %+v", r, want)
	}
}

func TestVerifyMatchesRequestsToRoutesAndLetsTheRestLeave(t *testing.T) {
	r, err := Verify([]Surface{
		{Service: "herald", Calls: []string{
			"POST /api/render/text", "GET /documents/{}", "GET /elsewhere", "GET payments/{}",
			"POST https://api.acquirer.test/v3/payments", "GET external",
		}},
		{Service: "renderer", Serves: []string{"POST /api/render/text", "GET /documents/{id}"}},
		{Service: "archive", Serves: []string{"GET /documents/{id}"}},
		{Service: "media", Serves: []string{"GET /*"}},
		{Service: "apostle", Serves: []string{"GET /admin/users/{userId}"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := Report{
		Ambiguous: []string{"GET /documents/{} (by herald, routed by archive, renderer)"},
		Outside: []string{
			"GET /elsewhere (by herald)",
			"GET external (by herald)",
			"GET payments/{} (by herald)",
			"POST https://api.acquirer.test/v3/payments (by herald)",
		},
	}
	if !reflect.DeepEqual(r, want) {
		t.Errorf("got  %+v\nwant %+v", r, want)
	}
}

func TestRouteMatching(t *testing.T) {
	for _, c := range []struct {
		route, request string
		want           bool
	}{
		{"GET /users/{id}", "GET /users/{}", true},
		{"GET /users/{id}", "GET /users/me", true},
		{"GET /users/me", "GET /users/{}", true},
		{"GET /users/{id}", "POST /users/{}", false},
		{"GET /*", "GET /a/b/c", true},
		{"GET /files/*", "GET /files", false},
		{"GET /users", "GET /users/{}", false},
		{"POST /api/render/text", "POST /api/render/template", false},
	} {
		if got := routeMatches(c.route, c.request); got != c.want {
			t.Errorf("routeMatches(%q, %q) = %v, want %v", c.route, c.request, got, c.want)
		}
	}
}

func TestReceiversAndSenders(t *testing.T) {
	set := []Surface{
		{Service: "dungeon", Publishes: []string{"contracts.moved"}, Calls: []string{"/shark.Pay/Charge", "POST /api/render/text"}},
		{Service: "bastion", Consumes: []string{"contracts.moved"}},
		{Service: "herald", Consumes: []string{"#"}},
		{Service: "shark", Serves: []string{"/shark.Pay/Charge", "GET /*"}},
		{Service: "renderer", Serves: []string{"POST /api/render/{kind}"}},
	}
	for _, c := range []struct {
		kind, name  string
		exact, wild []string
	}{
		{"event", "contracts.moved", []string{"bastion"}, []string{"herald"}},
		{"grpc", "/shark.Pay/Charge", []string{"shark"}, nil},
		{"http", "POST /api/render/text", []string{"renderer"}, nil},
		{"http", "GET /anything", nil, nil},
	} {
		exact, wild := Receivers(set, c.kind, c.name)
		if !reflect.DeepEqual(exact, c.exact) || !reflect.DeepEqual(wild, c.wild) {
			t.Errorf("Receivers(%s %s) = %v, %v; want %v, %v", c.kind, c.name, exact, wild, c.exact, c.wild)
		}
	}
	if got := Senders(set, "http", "POST /api/render/{kind}"); !reflect.DeepEqual(got, []string{"dungeon"}) {
		t.Errorf("Senders(http) = %v, want [dungeon]", got)
	}
	if got := Senders(set, "event", "contracts.moved"); !reflect.DeepEqual(got, []string{"dungeon"}) {
		t.Errorf("Senders(event) = %v, want [dungeon]", got)
	}
}

func TestVerifyRefusesAServiceTwice(t *testing.T) {
	if _, err := Verify([]Surface{{Service: "a"}, {Service: "a"}}); err == nil {
		t.Fatal("want a refusal of the duplicate")
	}
}

func TestTopicMatching(t *testing.T) {
	for _, c := range []struct {
		pattern, key string
		want         bool
	}{
		{"#", "a.b.c", true},
		{"a.#", "a", true},
		{"a.#", "a.b.c", true},
		{"a.*", "a.b", true},
		{"a.*", "a.b.c", false},
		{"a.*", "a", false},
		{"*.b.#", "a.b", true},
		{"a.b", "a.c", false},
	} {
		if got := matches(c.pattern, c.key); got != c.want {
			t.Errorf("matches(%q, %q) = %v, want %v", c.pattern, c.key, got, c.want)
		}
	}
}
