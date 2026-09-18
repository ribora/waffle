package http

import (
	"errors"
	"fmt"

	"gitlab.com/gorib/http/server"
)

var ErrNoRouters = errors.New("http: no server.Router is registered; register one with di.Define(NewRouter, di.Tag(http.RouterTag))")

// chi keeps the last registration for a pattern and says nothing, so without
// this check the domain that loses is simply unreachable.
func Setup(mux server.Mux, routers []server.Router) error {
	if len(routers) == 0 {
		return ErrNoRouters
	}
	owner := map[string]string{}
	for _, router := range routers {
		name := fmt.Sprintf("%T", router)
		// Group shares this slice, so the tail is everything the router claimed.
		before := len(mux.Routes())
		router.Setup(mux)
		for _, route := range mux.Routes()[before:] {
			key := route.Method + " " + route.Path
			if held, taken := owner[key]; taken {
				return fmt.Errorf("http: %s is registered by %s and by %s", key, held, name)
			}
			owner[key] = name
		}
	}
	return nil
}
