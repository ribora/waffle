package http

import (
	httpServer "gitlab.com/gorib/http/server"
	"gitlab.com/gorib/http/server/middleware"
	"gitlab.com/gorib/pry"
)

type {{ title .Domain }}Management interface {
	// Methods consumed by the routes go here.
}

func NewRouter(
	{{ camel .Domain }}Management {{ title .Domain }}Management,
	standard *middleware.Standard,
	logger pry.Logger,
) httpServer.Router {
	return &router{
		{{ camel .Domain }}Management: {{ camel .Domain }}Management,
		standard:                standard,
		logger:                  logger,
	}
}

type router struct {
	{{ camel .Domain }}Management {{ title .Domain }}Management
	standard                 *middleware.Standard
	logger                   pry.Logger
}

func (r *router) Setup(mux httpServer.Mux) {
	mux.Use(r.standard.All()...)

	// Register routes here, e.g.:
	//   mux.Get("/{{ .Domain }}/{id}", httpServer.Handle(r.handler.Get))
}
