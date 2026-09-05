package grpc

import (
	"context"
	"net"

	baseGrpc "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	grpc "gitlab.com/gorib/grpc/server"
)

// The chain comes from UnaryChain, the same function the daemon uses: a harness
// that rebuilt it would be checking its own copy of the order.
func Serve(handlers []ServiceHandler, interceptors []grpc.Interceptor, streamInterceptors []grpc.StreamInterceptor) (*InProcess, error) {
	server := baseGrpc.NewServer(
		baseGrpc.ChainUnaryInterceptor(UnaryChain(interceptors)...),
		baseGrpc.ChainStreamInterceptor(StreamChain(streamInterceptors)...),
	)
	for _, handler := range handlers {
		handler.Start(server)
	}

	listener := bufconn.Listen(1024 * 1024)
	go func() { _ = server.Serve(listener) }()

	connection, err := baseGrpc.NewClient("passthrough:///bufconn",
		baseGrpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return listener.DialContext(ctx)
		}),
		baseGrpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		server.Stop()
		_ = listener.Close()
		return nil, err
	}

	return &InProcess{connection: connection, server: server, listener: listener}, nil
}

type InProcess struct {
	connection *baseGrpc.ClientConn
	server     *baseGrpc.Server
	listener   *bufconn.Listener
}

func (s *InProcess) Conn() baseGrpc.ClientConnInterface { return s.connection }

func (s *InProcess) Close() {
	_ = s.connection.Close()
	s.server.Stop()
	_ = s.listener.Close()
}
