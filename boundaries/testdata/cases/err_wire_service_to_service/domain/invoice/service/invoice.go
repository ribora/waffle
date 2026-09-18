package service

import (
	"context"

	"example.test/domain/invoice/model"
)

type Invoices struct{}

func NewInvoices() *Invoices { return &Invoices{} }

func (Invoices) Issue(_ context.Context, amount int) (*model.Invoice, error) {
	return &model.Invoice{Amount: amount}, nil
}

// Primitives only, so a neighbour can name it without importing anything of
// invoice's — the crossing an import check cannot see.
func (Invoices) Cancel(_ context.Context, invoiceId int) error { return nil }
