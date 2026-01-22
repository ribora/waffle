syntax = "proto3";

package {{ .Domain }};
option go_package = "../pb/{{ .Service }}/{{ .Domain }}";

service {{ title .Domain }}Service {
}
