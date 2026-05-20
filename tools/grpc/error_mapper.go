package grpc

import "google.golang.org/grpc/codes"

func NewErrorMap(mappers []ErrorMapper) ErrorMapper {
	em := errorsMap{}
	for _, m := range mappers {
		for k, v := range m.Errors() {
			em[k] = v
		}
	}
	return em
}

type errorsMap map[error]codes.Code

func (em errorsMap) Errors() map[error]codes.Code {
	return em
}
