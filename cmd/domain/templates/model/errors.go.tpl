package model

import (
	"errors"
)

var (
	Err{{ .TitleDomain }}NotFound = errors.New("{{ .Domain }} is not found")
)
