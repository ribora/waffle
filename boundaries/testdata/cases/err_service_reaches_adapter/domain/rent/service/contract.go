package service

import (
	"context"

	"example.test/domain/rent/adapter/postgres"
	"example.test/domain/rent/model"
)

type ContractRepository interface {
	Get(ctx context.Context, id int) (*model.Contract, error)
}

type Contracts struct {
	contracts ContractRepository
}

// The interface is declared right here, and then answered by name anyway: the
// service is welded to postgres and no test can replace it.
func NewContracts() Contracts {
	return Contracts{contracts: postgres.NewContractRepository()}
}
