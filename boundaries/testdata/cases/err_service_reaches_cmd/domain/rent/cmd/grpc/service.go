package grpc

import (
	"context"

	"example.test/domain/rent/model"
)

type ContractManagement interface {
	Contract(ctx context.Context, id int) (*model.Contract, error)
}

type ContractService struct {
	contracts ContractManagement
}
