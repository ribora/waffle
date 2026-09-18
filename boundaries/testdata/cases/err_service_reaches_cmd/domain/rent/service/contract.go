package service

import (
	"context"

	"example.test/domain/rent/cmd/grpc"
	"example.test/domain/rent/model"
)

type Contracts struct{}

// The handler's interface is the handler's; a service that implements it by name
// has turned the inversion around.
var _ grpc.ContractManagement = Contracts{}

func (Contracts) Contract(_ context.Context, id int) (*model.Contract, error) {
	return &model.Contract{Id: id}, nil
}
