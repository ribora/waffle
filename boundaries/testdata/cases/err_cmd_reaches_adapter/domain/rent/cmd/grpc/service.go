package grpc

import (
	"context"

	"example.test/domain/rent/adapter/postgres"
	"example.test/domain/rent/model"
)

type ContractService struct {
	contracts ContractRepository
}

type ContractRepository interface {
	Get(ctx context.Context, id int) (*model.Contract, error)
}

// Built here rather than taken from di, which is what welds the handler to
// postgres.
func NewContractService() ContractService {
	return ContractService{contracts: postgres.NewContractRepository()}
}
