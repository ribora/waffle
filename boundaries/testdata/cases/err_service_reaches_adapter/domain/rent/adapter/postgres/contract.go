package postgres

import (
	"context"

	"example.test/domain/rent/model"
)

type contractRepository struct{}

func NewContractRepository() *contractRepository { return &contractRepository{} }

func (contractRepository) Get(_ context.Context, id int) (*model.Contract, error) {
	return &model.Contract{Id: id}, nil
}
