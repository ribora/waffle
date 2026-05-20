package grpc

import (
	"context"
	"fmt"
	"net"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/pborman/getopt/v2"
	baseGrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	grpc "gitlab.com/gorib/grpc/server"
	"gitlab.com/gorib/pry"
	"gitlab.com/gorib/waffle/app"
)

type GrpcCommand interface {
	app.Command
}

type (
	Middleware       = grpc.Interceptor
	StreamMiddleware = grpc.StreamInterceptor
)

type ServiceHandler interface {
	Start(baseGrpc.ServiceRegistrar)
}

type Server interface {
	Serve(ctx context.Context) error
	Close() error
}

type ErrorMapper interface {
	Errors() map[error]codes.Code
}

type StreamErrorMapper interface {
	StreamErrors() map[error]codes.Code
}

func NewGrpcCommand(commands []ServiceHandler, interceptors []Middleware, streamInterceptors []StreamMiddleware, errMap *grpc.ErrorMap, command app.BaseCommand, addr net.Addr, logger pry.Logger) *grpcCommand {
	sortedUnary := append([]Middleware(nil), interceptors...)
	sort.SliceStable(sortedUnary, func(i, j int) bool { return sortedUnary[i].Order() < sortedUnary[j].Order() })
	mw := make([]baseGrpc.UnaryServerInterceptor, 0, len(sortedUnary))
	for _, ic := range sortedUnary {
		mw = append(mw, ic.Run())
		if em, ok := any(ic).(ErrorMapper); ok {
			for e, code := range em.Errors() {
				errMap.Unary[e] = code
			}
		}
	}
	sortedStream := append([]StreamMiddleware(nil), streamInterceptors...)
	sort.SliceStable(sortedStream, func(i, j int) bool { return sortedStream[i].Order() < sortedStream[j].Order() })
	streamMw := make([]baseGrpc.StreamServerInterceptor, 0, len(sortedStream))
	for _, ic := range sortedStream {
		streamMw = append(streamMw, ic.Run())
		if em, ok := any(ic).(StreamErrorMapper); ok {
			for e, code := range em.StreamErrors() {
				errMap.Stream[e] = code
			}
		}
	}
	return &grpcCommand{
		commands:    commands,
		middlewares: mw,
		streamMw:    streamMw,
		addr:        addr,
		BaseCommand: command,
		logger:      logger,
	}
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
	err := c.Usage()
	if err != nil {
		return err
	}

	c.connection = grpc.NewServer(c, c.middlewares, c.streamMw, c.addr, c.logger)
	return c.connection.Serve(ctx)
}

func (c *grpcCommand) Usage() error {
	var help bool
	getopt.FlagLong(&help, "help", 'h', "Display this help message")
	listen := getopt.StringLong("listen", 'l', "", "Listen for incoming connections on [host:port]", "host:port")
	getopt.Parse()
	if help {
		getopt.SetParameters("")
		_, _ = fmt.Fprintf(os.Stderr, "%s\n", c.Description())
		getopt.PrintUsage(os.Stderr)
		os.Exit(2)
	}
	if *listen != "" {
		parts := strings.Split(*listen, ":")
		if len(parts) > 2 {
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
				return fmt.Errorf("Invalid port number")
			}
		}
		c.addr, err = net.ResolveTCPAddr("tcp", net.JoinHostPort(dest, strconv.Itoa(int(port))))
	}
	return nil
}

func (c *grpcCommand) Close() error {
	if c.connection != nil {
		_ = c.connection.Close()
	}
	return nil
}
