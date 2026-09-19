package service

import (
	"context"

	"example.test/domain/rent/model"
)

type Contracts struct{}

func NewContracts() *Contracts { return &Contracts{} }

func (Contracts) Get(_ context.Context, id int) (*model.Contract, error) {
	return &model.Contract{Id: id}, nil
}
