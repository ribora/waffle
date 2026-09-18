package service

import (
	"context"

	"example.test/domain/rent/model"
)

type ContractRepository interface {
	Get(ctx context.Context, id int) (*model.Contract, error)
}

type Contracts struct {
	contracts ContractRepository
}

func NewContracts(contracts ContractRepository) *Contracts {
	return &Contracts{contracts: contracts}
}
