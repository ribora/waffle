package vault

import (
	"context"

	"example.test/domain/payment/model"
	vaultModel "example.test/domain/vault/model"
	"example.test/pkg/money"
)

type MethodManagement interface {
	Method(ctx context.Context, id string) (*vaultModel.Method, error)
}

type methodService struct {
	methods MethodManagement
}

func NewMethodService(methods MethodManagement) *methodService {
	return &methodService{methods: methods}
}

func (s *methodService) Method(ctx context.Context, id string) (*model.Method, error) {
	method, err := s.methods.Method(ctx, id)
	if err != nil {
		return nil, err
	}
	return &model.Method{Id: method.Id, Brand: money.Title(method.Brand)}, nil
}
