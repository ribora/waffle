package service

import (
	"context"

	"example.test/domain/invoice/model"
)

type Invoices struct{}

func (Invoices) Issue(_ context.Context, amount int) (*model.Invoice, error) {
	return &model.Invoice{Amount: amount}, nil
}
