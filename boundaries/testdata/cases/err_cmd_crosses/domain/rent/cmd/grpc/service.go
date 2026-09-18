package grpc

import (
	"context"

	invoiceModel "example.test/domain/invoice/model"
	"example.test/domain/rent/model"
)

type ContractManagement interface {
	Contract(ctx context.Context, id int) (*model.Contract, error)
}

type ContractService struct {
	contracts ContractManagement
}

func (s ContractService) Invoice(ctx context.Context, id int) (*invoiceModel.Invoice, error) {
	contract, err := s.contracts.Contract(ctx, id)
	if err != nil {
		return nil, err
	}
	return &invoiceModel.Invoice{Id: contract.Id}, nil
}
