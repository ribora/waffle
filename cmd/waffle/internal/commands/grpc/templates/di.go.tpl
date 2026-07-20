package di

import (
	{{ .Domain }}Grpc "{{ .Project }}/domain/{{ .Domain }}/cmd/grpc"

	"gitlab.com/gorib/di"
	grpcCommand "gitlab.com/gorib/waffle/tools/grpc"
)

func Init{{ title .Domain }}Grpc() {
	di.Define({{ .Domain }}Grpc.New{{ title .Domain }}Service, di.Tag(grpcCommand.ServiceTag))
	di.Define({{ .Domain }}Grpc.NewErrorHandler, di.Tag(grpcCommand.ErrorMapperTag))

	di.Wire[{{ .Domain }}Grpc.{{ title .Domain }}Management]({{ .Domain }}ManagementInstance)
}
