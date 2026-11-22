package http

import (
	"errors"
	"fmt"

	"github.com/ribora/http/server"
)

var ErrNoRouters = errors.New("http: no server.Router is registered; register one with di.Define(NewRouter, di.Tag(http.RouterTag))")

type Global []server.Middleware

func NewGlobal(layers []server.Middleware) Global { return layers }

// chi keeps the last registration for a pattern and says nothing, so without
// this check the domain that loses is simply unreachable.
func Setup(mux server.Mux, global Global, routers []server.Router) error {
	if len(routers) == 0 {
		return ErrNoRouters
	}
	for _, layer := range global {
		mux.Use(layer)
	}
	owner := map[string]string{}
	for _, router := range routers {
		name := fmt.Sprintf("%T", router)
		// Group shares this slice, so the tail is everything the router claimed.
		before := len(mux.Routes())
		router.Setup(mux.Group(""))
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
