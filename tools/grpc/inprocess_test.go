package grpc

import (
	"context"
	"strings"
	"sync"
	"testing"

	baseGrpc "google.golang.org/grpc"

	"gitlab.com/gorib/grpc/echo/pb"
	grpc "gitlab.com/gorib/grpc/server"
)

type echoService struct {
	pb.UnimplementedEchoServiceServer
	mu    sync.Mutex
	calls int
}

func (s *echoService) Check(context.Context, *pb.EchoRequest) (*pb.EchoResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	return &pb.EchoResponse{}, nil
}

func (s *echoService) Start(registrar baseGrpc.ServiceRegistrar) {
	pb.RegisterEchoServiceServer(registrar, s)
}

func (s *echoService) Calls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

type tag struct {
	name  string
	order int
	log   *entered
}

func (t tag) Name() string { return t.name }
func (t tag) Order() int   { return t.order }

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
	service := &echoService{}
	server, err := Serve([]ServiceHandler{service}, nil, nil)
	if err != nil {
		t.Fatalf("serve: %v", err)
	}
	defer server.Close()

	_, err = pb.NewEchoServiceClient(server.Conn()).Check(t.Context(), &pb.EchoRequest{})

	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if service.Calls() != 1 {
		t.Fatalf("calls: %d, want 1", service.Calls())
	}
}

func TestServeRunsTheInterceptorsByOrder(t *testing.T) {
	log := &entered{}
	server, err := Serve(
		[]ServiceHandler{&echoService{}},
		[]grpc.Interceptor{
			tag{name: "inner", order: 30, log: log},
			tag{name: "outer", order: 10, log: log},
			tag{name: "middle", order: 20, log: log},
		},
		nil,
	)
	if err != nil {
		t.Fatalf("serve: %v", err)
	}
	defer server.Close()

	if _, err = pb.NewEchoServiceClient(server.Conn()).Check(t.Context(), &pb.EchoRequest{}); err != nil {
		t.Fatalf("check: %v", err)
	}

	if got := log.String(); got != "outer middle inner" {
		t.Fatalf("entered: %q, want outer middle inner", got)
	}
}

func TestUnaryChainOrdersSmallestOutermost(t *testing.T) {
	log := &entered{}
	chain := UnaryChain([]grpc.Interceptor{
		tag{name: "second", order: 2, log: log},
		tag{name: "first", order: 1, log: log},
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

func TestUnaryChainLeavesTheInputAlone(t *testing.T) {
	log := &entered{}
	given := []grpc.Interceptor{
		tag{name: "second", order: 2, log: log},
		tag{name: "first", order: 1, log: log},
	}

	UnaryChain(given)

	if given[0].Name() != "second" {
		t.Fatalf("the input was reordered: %s", given[0].Name())
	}
}

func TestCloseIsSafeAfterServe(t *testing.T) {
	server, err := Serve([]ServiceHandler{&echoService{}}, nil, nil)
	if err != nil {
		t.Fatalf("serve: %v", err)
	}

	server.Close()

	if _, err = pb.NewEchoServiceClient(server.Conn()).Check(t.Context(), &pb.EchoRequest{}); err == nil {
		t.Fatal("the connection still answers after Close")
	}
}
