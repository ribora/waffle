package {{ .Package }}

//go:generate go tool goverter gen ./...

// goverter:converter
// goverter:output:file ./convert_goverter_gen.go
// goverter:ignoreUnexported
// goverter:output:format function
// goverter:extend {{ .Project }}/pkg/goverter:.*
type Convert interface {
	// One method per type pair, e.g. ExampleToProto(*model.Example) *proto.Example
	// Overrides and field mapping: see docs/conventions/dto-conversion.md.
}
