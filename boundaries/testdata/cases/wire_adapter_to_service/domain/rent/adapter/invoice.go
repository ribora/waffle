package adapter

import (
	"context"

	invoiceModel "example.test/domain/invoice/model"
	"example.test/domain/rent/model"
)

type InvoiceManagement interface {
	Issue(ctx context.Context, amount int) (*invoiceModel.Invoice, error)
}

type invoiceService struct {
	invoices InvoiceManagement
}

func NewInvoiceService(invoices InvoiceManagement) *invoiceService {
	return &invoiceService{invoices: invoices}
}

func (s *invoiceService) Issue(ctx context.Context, contract *model.Contract) (*model.Invoice, error) {
	issued, err := s.invoices.Issue(ctx, contract.Id)
	if err != nil {
		return nil, err
	}
	return &model.Invoice{Id: issued.Id, Amount: issued.Amount}, nil
}
