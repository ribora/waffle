package grpc

import (
	"context"

	"example.test/domain/rent/cmd/grpc/convert"
	"example.test/domain/rent/model"
	"example.test/domain/rent/pricing"
)

type ContractManagement interface {
	Contract(ctx context.Context, id int) (*model.Contract, error)
}

type ContractService struct {
	contracts ContractManagement
	tier      pricing.Tier
}

func (s ContractService) Id(ctx context.Context, id int) (int, error) {
	contract, err := s.contracts.Contract(ctx, id)
	if err != nil {
		return 0, err
	}
	return convert.ContractId(contract), nil
}
