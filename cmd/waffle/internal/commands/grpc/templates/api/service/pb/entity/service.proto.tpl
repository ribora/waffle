syntax = "proto3";

package {{ .Domain }};
option go_package = "../proto/{{ .Domain }}";

service {{ title .Domain }}Service {
}
