package model

import (
    "time"
)

type {{ title .Domain }} struct {
    Id        int
    CreatedAt time.Time
    UpdatedAt *time.Time
}
