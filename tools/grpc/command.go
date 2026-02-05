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
	"google.golang.org/grpc/codes"

	grpc "gitlab.com/gorib/grpc/server"
	"gitlab.com/gorib/pry"
	"gitlab.com/gorib/waffle/app"
)

type GrpcCommand interface {
	app.Command
}

type Middleware interface {
	Run() baseGrpc.UnaryServerInterceptor
}

type StreamMiddleware interface {
	Run() baseGrpc.StreamServerInterceptor
}

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

func NewGrpcCommand(commands []ServiceHandler, middlewares []Middleware, streamMiddlewares []StreamMiddleware, command app.BaseCommand, addr net.Addr, logger pry.Logger) *grpcCommand {
	mw := make([]baseGrpc.UnaryServerInterceptor, 0, len(middlewares))
	errMapper := make(map[error]codes.Code)
	for _, middleware := range middlewares {
		mw = append(mw, middleware.Run())
		if em, ok := any(mw).(ErrorMapper); ok {
			for e, code := range em.Errors() {
				errMapper[e] = code
			}
		}
	}
	errMapperStream := make(map[error]codes.Code)
	streamMw := make([]baseGrpc.StreamServerInterceptor, 0, len(middlewares))
	for _, middleware := range streamMiddlewares {
		streamMw = append(streamMw, middleware.Run())
		if em, ok := any(mw).(StreamErrorMapper); ok {
			for e, code := range em.StreamErrors() {
				errMapperStream[e] = code
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
	commands        []ServiceHandler
	middlewares     []baseGrpc.UnaryServerInterceptor
	streamMw        []baseGrpc.StreamServerInterceptor
	errMapper       map[error]codes.Code
	errMapperStream map[error]codes.Code
	addr            net.Addr
	connection      Server
	logger          pry.Logger
}

func (c *grpcCommand) Errors() map[error]codes.Code {
	return c.errMapper
}

func (c *grpcCommand) StreamErrors() map[error]codes.Code {
	return c.errMapperStream
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
