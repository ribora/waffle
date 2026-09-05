package grpc

import (
	"cmp"
	"context"
	"fmt"
	"net"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/pborman/getopt/v2"
	baseGrpc "google.golang.org/grpc"

	grpc "gitlab.com/gorib/grpc/server"
	"gitlab.com/gorib/pry"
	"gitlab.com/gorib/waffle/app"
)

type (
	serviceTag          struct{}
	middlewareTag       struct{}
	streamMiddlewareTag struct{}
	errorMapperTag      struct{}
)

var (
	ServiceTag          = serviceTag{}
	MiddlewareTag       = middlewareTag{}
	StreamMiddlewareTag = streamMiddlewareTag{}
	ErrorMapperTag      = errorMapperTag{}
)

type GrpcCommand interface {
	app.Command
}

type ServiceHandler interface {
	Start(baseGrpc.ServiceRegistrar)
}

type Server interface {
	Serve(ctx context.Context) error
	Close() error
}

func NewGrpcCommand(commands []ServiceHandler, interceptors []grpc.Interceptor, streamInterceptors []grpc.StreamInterceptor, command app.BaseCommand, addr net.Addr, logger pry.Logger) *grpcCommand {
	return &grpcCommand{
		commands:    commands,
		middlewares: UnaryChain(interceptors),
		streamMw:    StreamChain(streamInterceptors),
		addr:        addr,
		BaseCommand: command,
		logger:      logger,
	}
}

// Ascending Order(), smaller outermost. Exported so an in-process test server
// chains them identically — getting this order wrong has already cost one bug.
func UnaryChain(interceptors []grpc.Interceptor) []baseGrpc.UnaryServerInterceptor {
	return chain[baseGrpc.UnaryServerInterceptor](interceptors)
}

func StreamChain(interceptors []grpc.StreamInterceptor) []baseGrpc.StreamServerInterceptor {
	return chain[baseGrpc.StreamServerInterceptor](interceptors)
}

type ordered[T any] interface {
	Order() int
	Run() T
}

func chain[T any, I ordered[T]](interceptors []I) []T {
	sorted := slices.Clone(interceptors)
	slices.SortStableFunc(sorted, func(a, b I) int { return cmp.Compare(a.Order(), b.Order()) })
	built := make([]T, 0, len(sorted))
	for _, interceptor := range sorted {
		built = append(built, interceptor.Run())
	}
	return built
}

type grpcCommand struct {
	app.BaseCommand
	commands    []ServiceHandler
	middlewares []baseGrpc.UnaryServerInterceptor
	streamMw    []baseGrpc.StreamServerInterceptor
	addr        net.Addr
	connection  Server
	logger      pry.Logger
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

	c.connection = grpc.NewServer(c, c.middlewares, c.streamMw, c.addr, c.logger)
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
	if c.connection != nil {
		_ = c.connection.Close()
	}
	return nil
}
