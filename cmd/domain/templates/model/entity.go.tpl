package model

import (
    "time"
)

type {{ .TitleDomain }} struct {
    Id        int
    CreatedAt time.Time
    UpdatedAt *time.Time
}
