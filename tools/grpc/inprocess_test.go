package grpc

import (
	"context"
	"strings"
	"sync"
	"testing"

	baseGrpc "google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"

	grpc "github.com/ribora/grpc/server"
)

// The health service stands in for any service at all: these tests are about the
// interceptor chain and the connection, not about what is being called.
type healthService struct {
	grpc_health_v1.UnimplementedHealthServer
	mu    sync.Mutex
	calls int
}

func (s *healthService) Check(context.Context, *grpc_health_v1.HealthCheckRequest) (*grpc_health_v1.HealthCheckResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	return &grpc_health_v1.HealthCheckResponse{}, nil
}

func (s *healthService) Start(registrar baseGrpc.ServiceRegistrar) {
	grpc_health_v1.RegisterHealthServer(registrar, s)
}

func (s *healthService) Calls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

type tag struct {
	name string
	log  *entered
}

func (t tag) Run() baseGrpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *baseGrpc.UnaryServerInfo, handler baseGrpc.UnaryHandler) (any, error) {
		t.log.add(t.name)
		return handler(ctx, req)
	}
}

type entered struct {
	mu    sync.Mutex
	names []string
}

func (e *entered) add(name string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.names = append(e.names, name)
}

func (e *entered) String() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return strings.Join(e.names, " ")
}

func TestServeReachesTheHandlerOverAConnection(t *testing.T) {
	service := &healthService{}
	server, err := Serve([]ServiceHandler{service}, nil, nil)
	if err != nil {
		t.Fatalf("serve: %v", err)
	}
	defer server.Close()

	_, err = grpc_health_v1.NewHealthClient(server.Conn()).Check(t.Context(), &grpc_health_v1.HealthCheckRequest{})

	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if service.Calls() != 1 {
		t.Fatalf("calls: %d, want 1", service.Calls())
	}
}

func TestServeRunsTheInterceptorsInTheOrderGiven(t *testing.T) {
	log := &entered{}
	server, err := Serve(
		[]ServiceHandler{&healthService{}},
		grpc.Interceptors{
			tag{name: "outer", log: log},
			tag{name: "middle", log: log},
			tag{name: "inner", log: log},
		},
		nil,
	)
	if err != nil {
		t.Fatalf("serve: %v", err)
	}
	defer server.Close()

	if _, err = grpc_health_v1.NewHealthClient(server.Conn()).Check(t.Context(), &grpc_health_v1.HealthCheckRequest{}); err != nil {
		t.Fatalf("check: %v", err)
	}

	if got := log.String(); got != "outer middle inner" {
		t.Fatalf("entered: %q, want outer middle inner", got)
	}
}

func TestUnaryChainKeepsTheOrderGiven(t *testing.T) {
	log := &entered{}
	chain := UnaryChain(grpc.Interceptors{
		tag{name: "first", log: log},
		tag{name: "second", log: log},
	})

	if len(chain) != 2 {
		t.Fatalf("chain: %d, want 2", len(chain))
	}
	for _, interceptor := range chain {
		_, _ = interceptor(t.Context(), nil, nil, func(context.Context, any) (any, error) { return nil, nil })
	}
	if got := log.String(); got != "first second" {
		t.Fatalf("entered: %q", got)
	}
}

func TestCloseIsSafeAfterServe(t *testing.T) {
	server, err := Serve([]ServiceHandler{&healthService{}}, nil, nil)
	if err != nil {
		t.Fatalf("serve: %v", err)
	}

	server.Close()

	if _, err = grpc_health_v1.NewHealthClient(server.Conn()).Check(t.Context(), &grpc_health_v1.HealthCheckRequest{}); err == nil {
		t.Fatal("the connection still answers after Close")
	}
}
