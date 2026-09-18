package service

import (
	"context"

	"example.test/domain/rent/model"
)

// Primitives only, so nothing of invoice's is imported anywhere in rent.
type InvoiceCancelling interface {
	Cancel(ctx context.Context, invoiceId int) error
}

type Contracts struct {
	invoices InvoiceCancelling
}

func (c Contracts) Terminate(ctx context.Context, contract *model.Contract) error {
	return c.invoices.Cancel(ctx, contract.InvoiceId)
}
