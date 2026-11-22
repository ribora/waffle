package grpc

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/pborman/getopt/v2"
	baseGrpc "google.golang.org/grpc"

	grpc "github.com/ribora/grpc/server"
	"github.com/ribora/pry"
	"github.com/ribora/waffle/app"
)

type (
	serviceTag     struct{}
	errorMapperTag struct{}
)

var (
	ServiceTag     = serviceTag{}
	ErrorMapperTag = errorMapperTag{}
)

type GrpcCommand interface {
	app.Command
}

type ServiceHandler interface {
	Start(baseGrpc.ServiceRegistrar)
}

type Drainer interface {
	Drain()
}

type Server interface {
	Serve(ctx context.Context) error
	Close() error
}

func NewGrpcCommand(commands []ServiceHandler, interceptors grpc.Interceptors, streamInterceptors grpc.StreamInterceptors, command app.BaseCommand, addr net.Addr, maxRecvMsgSize int, logger pry.Logger) *grpcCommand {
	return &grpcCommand{
		commands:       commands,
		middlewares:    UnaryChain(interceptors),
		streamMw:       StreamChain(streamInterceptors),
		addr:           addr,
		maxRecvMsgSize: maxRecvMsgSize,
		BaseCommand:    command,
		logger:         logger,
	}
}

func NewInterceptors(interceptors []grpc.Interceptor) grpc.Interceptors { return interceptors }

func NewStreamInterceptors(interceptors []grpc.StreamInterceptor) grpc.StreamInterceptors {
	return interceptors
}

// Exported so an in-process test server builds the chain the same way.
func UnaryChain(interceptors grpc.Interceptors) []baseGrpc.UnaryServerInterceptor {
	built := make([]baseGrpc.UnaryServerInterceptor, 0, len(interceptors))
	for _, interceptor := range interceptors {
		built = append(built, interceptor.Run())
	}
	return built
}

func StreamChain(interceptors grpc.StreamInterceptors) []baseGrpc.StreamServerInterceptor {
	built := make([]baseGrpc.StreamServerInterceptor, 0, len(interceptors))
	for _, interceptor := range interceptors {
		built = append(built, interceptor.Run())
	}
	return built
}

type grpcCommand struct {
	app.BaseCommand
	commands       []ServiceHandler
	middlewares    []baseGrpc.UnaryServerInterceptor
	streamMw       []baseGrpc.StreamServerInterceptor
	addr           net.Addr
	maxRecvMsgSize int
	connection     Server
	logger         pry.Logger
}

func (c *grpcCommand) Setup(server *baseGrpc.Server) {
	for _, service := range c.commands {
		service.Start(server)
	}
}

func (c *grpcCommand) Run(ctx context.Context) error {
	err := c.Usage(ctx)
	if err != nil {
		return err
	}

	c.connection = grpc.NewServer(c, c.middlewares, c.streamMw, c.addr, c.logger, baseGrpc.MaxRecvMsgSize(c.maxRecvMsgSize))
	return c.connection.Serve(ctx)
}

func (c *grpcCommand) Usage(ctx context.Context) error {
	opts := getopt.New()
	var help bool
	opts.FlagLong(&help, "help", 'h', "Display this help message")
	listen := opts.StringLong("listen", 'l', "", "Listen for incoming connections on [host:port]", "host:port")
	if err := opts.Getopt(app.Args(ctx), nil); err != nil {
		return err
	}
	if help {
		opts.SetParameters("")
		_, _ = fmt.Fprintf(os.Stderr, "%s\n", c.Description())
		opts.PrintUsage(os.Stderr)
		return app.ErrUsage
	}
	if *listen != "" {
		parts := strings.Split(*listen, ":")
		if len(parts) > 2 {
			//nolint:staticcheck
			return fmt.Errorf("Unsupported host format, expected host:port")
		}
		dest := parts[0]
		var port uint64 = 80
		var err error
		if len(parts) == 2 {
			port, err = strconv.ParseUint(parts[1], 10, 16)
			if err != nil {
				return err
			}
			if port == 0 {
				//nolint:staticcheck
				return fmt.Errorf("Invalid port number")
			}
		}
		c.addr, err = net.ResolveTCPAddr("tcp", net.JoinHostPort(dest, strconv.Itoa(int(port))))
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *grpcCommand) Daemon() {}

func (c *grpcCommand) Close() error {
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		if c.connection != nil {
			_ = c.connection.Close()
		}
	}()
	// The server's Close waits for every running call, and a drained one ends only while it waits.
	for _, service := range c.commands {
		if drainer, ok := service.(Drainer); ok {
			drainer.Drain()
		}
	}
	<-stopped
	return nil
}
