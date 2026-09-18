package service

import (
	"context"

	"example.test/domain/rent/model"
)

// Declared here and never named: adapter/rf implements it without either package
// importing the other.
type Policy interface {
	Country(ctx context.Context) string
}

type Contracts struct {
	policy Policy
}

func (c Contracts) Country(ctx context.Context, _ *model.Contract) string {
	return c.policy.Country(ctx)
}
