package grpc

import (
	proto "{{ .ApiImport }}/{{ .Domain }}"
	"google.golang.org/grpc"
)

type {{ title .Domain }}Management interface {
}

func New{{ title .Domain }}Service({{ .Domain }}Management {{ title .Domain }}Management) *{{ .Domain }}Service {
	return &{{ .Domain }}Service{ {{- .Domain }}Management: {{ .Domain }}Management}
}

type {{ .Domain }}Service struct {
	proto.{{ title .Domain }}ServiceServer
	{{ .Domain }}Management {{ title .Domain }}Management
}

func (s *{{ .Domain }}Service) Start(server grpc.ServiceRegistrar) {
	proto.Register{{ title .Domain }}ServiceServer(server, s)
}
