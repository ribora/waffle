# Project conventions for waffle generators. Missing file or missing keys
# fall back to the same defaults, so this file is documentation as much as
# configuration.

# Go import path of generated protobuf packages (<api_import>/<domain>).
api_import: {{ .Project }}/api/pb

# Where make:domain:grpc puts .proto sources. A path outside the module
# (e.g. ../api/proto in a monorepo) skips local proto generation.
proto_dir: api/proto

# goverter — generated convert package in cmd/<transport>/convert (default);
# it refuses to generate on a field it cannot map, so a field added to a model
# cannot silently stop reaching the wire.
# dto — handwritten dto.go next to the handler. Loses that check.
mapper: goverter
