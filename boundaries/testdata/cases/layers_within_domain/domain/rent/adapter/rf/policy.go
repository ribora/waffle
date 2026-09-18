package rf

import "context"

type Policy struct {
	country string
}

func NewPolicy() *Policy { return &Policy{country: "rf"} }

func (p *Policy) Country(_ context.Context) string { return p.country }
