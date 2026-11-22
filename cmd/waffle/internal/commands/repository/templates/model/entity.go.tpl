package model

import (
	"time"

	"github.com/ribora/failure"
)

var (
	Err{{ title .Entity }}NotFound = failure.New("{{ .Entity }} is not found")
)

type {{ title .Entity }} struct {
	Id        int
	CreatedAt time.Time
	UpdatedAt *time.Time
}
