package http

import (
	"github.com/gofiber/fiber/v2"

	httpServer "gitlab.com/gorib/http/server"
	"gitlab.com/gorib/pry"
	"gitlab.com/gorib/waffle/tools/http"
)

type {{ title .Domain }}Management interface {
	// Methods consumed by the routes go here.
}

func NewRouter(
	{{ .Domain }}Management {{ title .Domain }}Management,
	logger pry.Logger,
) httpServer.Router {
	return &router{
		{{ .Domain }}Management: {{ .Domain }}Management,
		logger:                   logger,
	}
}

type router struct {
	{{ .Domain }}Management {{ title .Domain }}Management
	logger pry.Logger
}

func (r *router) Setup(app *fiber.App, mws http.Middlewares) {
	// Register routes here.
}
