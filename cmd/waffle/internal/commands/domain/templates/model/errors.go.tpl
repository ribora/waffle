package model

import (
	"gitlab.com/gorib/failure"
)

var (
	Err{{ title .Domain }}NotFound = failure.New("{{ .Domain }} is not found")
)
