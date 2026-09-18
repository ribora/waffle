package sberbank

import "example.test/domain/rent/adapter/rf"

type gateway struct {
	policy *rf.Policy
}

func NewGateway(policy *rf.Policy) *gateway { return &gateway{policy: policy} }
