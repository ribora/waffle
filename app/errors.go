package app

import (
	"errors"

	"github.com/ribora/failure"
)

var ErrUsage = errors.New("usage")

const otherErrorType = "_OTHER"

func errorType(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, ErrUsage) {
		return "usage"
	}
	if fe, ok := failure.FromError(err); ok {
		return fe.Type()
	}
	return otherErrorType
}
