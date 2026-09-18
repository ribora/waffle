package service

import (
	"context"

	"example.test/domain/rent/model"
)

type Invoicing interface {
	Issue(ctx context.Context, contract *model.Contract) (*model.Invoice, error)
}

type Contracts struct {
	invoicing Invoicing
}

func (c Contracts) Bill(ctx context.Context, contract *model.Contract) (*model.Invoice, error) {
	return c.invoicing.Issue(ctx, contract)
}
