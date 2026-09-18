package adapter

import (
	"context"

	invoiceService "example.test/domain/invoice/service"
	"example.test/domain/rent/model"
)

type invoiceAdapter struct {
	invoices invoiceService.Invoices
}

func (a invoiceAdapter) Issue(ctx context.Context, contract *model.Contract) (*model.Invoice, error) {
	issued, err := a.invoices.Issue(ctx, contract.Id)
	if err != nil {
		return nil, err
	}
	return &model.Invoice{Id: issued.Id, Amount: issued.Amount}, nil
}
