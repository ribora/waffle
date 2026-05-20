package http

import (
	"fmt"

	"github.com/gofiber/fiber/v2"

	httpServer "gitlab.com/gorib/http/server"
)

// Re-exports so user code keeps using whttp.Middleware and friends without
// importing gorib/http/server directly.
type (
	Middleware = httpServer.Middleware
	Globaler   = httpServer.Globaler
)

type MiddlewareList interface {
	Get(name string) fiber.Handler
}

func NewMiddlewareList(middlewares []Middleware) middlewareList {
	mwList := make(map[string]fiber.Handler)
	for _, middleware := range middlewares {
		mwList[middleware.Name()] = middleware.Run()
	}
	return mwList
}

type middlewareList map[string]fiber.Handler

func (m middlewareList) Get(name string) fiber.Handler {
	if mw, found := m[name]; found {
		return mw
	}
	panic(fmt.Errorf("middleware is not found: '%s'", name))
}
