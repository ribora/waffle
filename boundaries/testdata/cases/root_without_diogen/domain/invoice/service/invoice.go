package service

import (
	"context"

	"example.test/domain/invoice/model"
)

type InvoiceRepository interface {
	Get(ctx context.Context, id int) (*model.Invoice, error)
}

type Invoices struct {
	invoices InvoiceRepository
}

func NewInvoices(invoices InvoiceRepository) *Invoices {
	return &Invoices{invoices: invoices}
}
