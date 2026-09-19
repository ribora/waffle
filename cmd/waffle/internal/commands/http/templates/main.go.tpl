package http

import (
	httpServer "gitlab.com/gorib/http/server"
	"gitlab.com/gorib/pry"
)

type {{ title .Domain }}Management interface {
	// Methods consumed by the routes go here.
}

func NewRouter(
	{{ camel .Domain }}Management {{ title .Domain }}Management,
	logger pry.Logger,
) httpServer.Router {
	return &router{
		{{ camel .Domain }}Management: {{ camel .Domain }}Management,
		logger:                  logger,
	}
}

type router struct {
	{{ camel .Domain }}Management {{ title .Domain }}Management
	logger                   pry.Logger
}

func (r *router) Setup(mux httpServer.Mux) {
	// Register routes here, e.g.:
	//   mux.Get("/{{ .Domain }}/{id}", httpServer.Handle(r.handler.Get))
}
