package grpc

import (
	proto "{{ .ApiImport }}/{{ .Domain }}"
	"google.golang.org/grpc"
)

type {{ title .Domain }}Management interface {
}

func New{{ title .Domain }}Service({{ camel .Domain }}Management {{ title .Domain }}Management) *{{ camel .Domain }}Service {
	return &{{ camel .Domain }}Service{ {{- camel .Domain }}Management: {{ camel .Domain }}Management}
}

type {{ camel .Domain }}Service struct {
	proto.{{ title .Domain }}ServiceServer
	{{ camel .Domain }}Management {{ title .Domain }}Management
}

func (s *{{ camel .Domain }}Service) Start(server grpc.ServiceRegistrar) {
	proto.Register{{ title .Domain }}ServiceServer(server, s)
}
