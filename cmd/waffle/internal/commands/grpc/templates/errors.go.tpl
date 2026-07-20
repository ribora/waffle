package grpc

import (
	"google.golang.org/grpc/codes"
)

func NewErrorHandler() *errorHandler {
	return &errorHandler{}
}

type errorHandler struct{}

func (h *errorHandler) Errors() map[error]codes.Code {
	return map[error]codes.Code{}
}
