package grpc

import (
	"google.golang.org/grpc"
)

type Middleware interface {
	Run() grpc.UnaryServerInterceptor
}

type MiddlewareList interface {
	Mw() []grpc.UnaryServerInterceptor
}

func NewMiddlewareList(middlewares []Middleware) MiddlewareList {
	mw := make([]grpc.UnaryServerInterceptor, 0, len(middlewares))
	for _, middleware := range middlewares {
		mw = append(mw, middleware.Run())
	}
	return &middlewareList{middlewares: mw}
}

type middlewareList struct {
	middlewares []grpc.UnaryServerInterceptor
}

func (m *middlewareList) Mw() []grpc.UnaryServerInterceptor {
	return m.middlewares
}
