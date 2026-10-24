package tap

import (
	"context"
	"slices"
	"testing"
)

func TestAMessageReachesTheListenerUntilItStops(t *testing.T) {
	var heard []string
	Listen(func(dir, key string, body []byte) { heard = append(heard, dir+" "+key+" "+string(body)) })
	Message("out", "invoices.paid", []byte("{}"))
	Listen(nil)
	Message("in", "invoices.paid", nil)

	if want := []string{"out invoices.paid {}"}; !slices.Equal(heard, want) {
		t.Errorf("heard %v, want %v", heard, want)
	}
}

func TestARouteReachesOnlyWhoAskedForIt(t *testing.T) {
	route := new(string)
	Route(WithRoute(context.Background(), route), "/orders/{id}")
	Route(context.Background(), "/elsewhere")

	if *route != "/orders/{id}" {
		t.Errorf("route = %q", *route)
	}
}
