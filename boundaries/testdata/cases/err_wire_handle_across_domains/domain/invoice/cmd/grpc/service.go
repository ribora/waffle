package grpc

import (
	"context"

	"example.test/domain/invoice/model"
)

type InvoiceManagement interface {
	Issue(ctx context.Context, amount int) (*model.Invoice, error)
}

type InvoiceService struct {
	invoices InvoiceManagement
}
