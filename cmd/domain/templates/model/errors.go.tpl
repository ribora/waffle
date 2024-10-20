package model

import (
	"errors"
)

var (
	Err{{ title .Domain }}NotFound = errors.New("{{ .Domain }} is not found")
)
