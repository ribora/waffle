package config

import (
    proto "{{ .Project }}/api/{{ .Domain }}"
    grpc{{ title .Domain }} "{{ .Project }}/domain/{{ .Domain }}/cmd/grpc/{{ .Domain }}"

    "gitlab.com/gorib/di"
)

func Init{{ title .Domain }}Grpc() {
    di.MustWire[proto.{{ title .Domain }}ServiceServer](grpc{{ title .Domain }}.New{{ title .Domain }}Service, di.Tag(grpcCommand.ServiceTag))

    di.Alias[grpc{{ title .Domain }}.{{ title .Domain }}Management](),
}
