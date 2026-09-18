package postgres

import (
	"context"

	"example.test/domain/invoice/model"
)

type invoiceRepository struct{}

func NewInvoiceRepository() *invoiceRepository { return &invoiceRepository{} }

func (invoiceRepository) Get(_ context.Context, id int) (*model.Invoice, error) {
	return &model.Invoice{Id: id}, nil
}
