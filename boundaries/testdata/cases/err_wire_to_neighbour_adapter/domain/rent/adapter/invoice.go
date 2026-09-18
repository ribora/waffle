package adapter

import (
	"context"

	invoiceModel "example.test/domain/invoice/model"
	"example.test/domain/rent/model"
)

type Invoices interface {
	Get(ctx context.Context, id int) (*invoiceModel.Invoice, error)
}

type invoiceService struct {
	invoices Invoices
}

func (s *invoiceService) Invoice(ctx context.Context, id int) (*model.Invoice, error) {
	invoice, err := s.invoices.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return &model.Invoice{Id: invoice.Id, Amount: invoice.Amount}, nil
}
