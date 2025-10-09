package grpc

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/pborman/getopt/v2"
	baseGrpc "google.golang.org/grpc"

	"gitlab.com/gorib/pry"
	"gitlab.com/gorib/server/grpc"
	"gitlab.com/gorib/server/host"
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

func NewGrpcCommand(commands []ServiceHandler, middlewares []Middleware, streamMiddlewares []StreamMiddleware, command app.BaseCommand, addr net.Addr, logger pry.Logger) *grpcCommand {
	mw := make([]baseGrpc.UnaryServerInterceptor, 0, len(middlewares))
	for _, middleware := range middlewares {
		mw = append(mw, middleware.Run())
	}
	streamMw := make([]baseGrpc.StreamServerInterceptor, 0, len(middlewares))
	for _, middleware := range streamMiddlewares {
		streamMw = append(streamMw, middleware.Run())
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
		port := "80"
		if len(parts) == 2 {
			port = parts[1]
		}
		var err error
		c.addr, err = host.NewListener("tcp", dest, port)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *grpcCommand) Close() error {
	if c.connection != nil {
		_ = c.connection.Close()
	}
	return nil
}
