package service

import (
	"context"

	"example.test/domain/invoice/model"
)

type invoiceManagement struct{}

func NewInvoiceManagement() *invoiceManagement { return &invoiceManagement{} }

func (invoiceManagement) Issue(_ context.Context, amount int) (*model.Invoice, error) {
	return &model.Invoice{Amount: amount}, nil
}

func (invoiceManagement) Cancel(_ context.Context, invoiceId int) error { return nil }
