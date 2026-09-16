package di

import (
	{{ camel .Domain }}Grpc "{{ .Project }}/domain/{{ .Domain }}/cmd/grpc"

	"gitlab.com/gorib/di"
	grpcCommand "gitlab.com/gorib/waffle/tools/grpc"
)

func Init{{ title .Domain }}Grpc() {
	di.Define({{ camel .Domain }}Grpc.New{{ title .Domain }}Service, di.Tag(grpcCommand.ServiceTag))
	di.Define({{ camel .Domain }}Grpc.NewErrorHandler, di.Tag(grpcCommand.ErrorMapperTag))

	di.Wire[{{ camel .Domain }}Grpc.{{ title .Domain }}Management]({{ camel .Domain }}ManagementInstance)
}
