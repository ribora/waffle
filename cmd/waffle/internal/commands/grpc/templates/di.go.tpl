package config

func Init{{ title .Domain }}Grpc() {
    err := di.Wire[{{ title .Domain }}ServiceServer](New{{ title .Domain }}Service, di.Tag(grpcCommand.ServiceTag))

    di.Alias[{{ title .Domain }}Management](),
}
