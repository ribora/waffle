package service

import (
	"context"

	invoiceModel "example.test/domain/invoice/model"
	"example.test/domain/rent/model"
)

type Contracts struct{}

func (Contracts) Bill(_ context.Context, contract *model.Contract) *invoiceModel.Invoice {
	return &invoiceModel.Invoice{Id: contract.Id}
}
