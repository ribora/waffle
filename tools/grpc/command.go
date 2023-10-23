package grpc

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/pborman/getopt/v2"
	baseGrpc "google.golang.org/grpc"

	"gitlab.com/gorib/server/grpc"
	"gitlab.com/gorib/server/host"
	"gitlab.com/gorib/waffle/app"
)

type GrpcCommand interface {
	app.Command
}

type ServiceHandler interface {
	Start(baseGrpc.ServiceRegistrar)
}

func NewGrpcCommand(commands []ServiceHandler, mw MiddlewareList, command app.BaseCommand, addr net.Addr) *grpcCommand {
	return &grpcCommand{
		commands:    commands,
		middlewares: mw.Mw(),
		addr:        addr,
		BaseCommand: command,
	}
}

type grpcCommand struct {
	app.BaseCommand
	commands    []ServiceHandler
	middlewares []baseGrpc.UnaryServerInterceptor
	addr        net.Addr
}

func (c *grpcCommand) Setup(server *baseGrpc.Server) {
	for _, service := range c.commands {
		service.Start(server)
	}
}

func (c *grpcCommand) Run(ctx context.Context) {
	c.Usage()

	server := grpc.NewServer(c, c.middlewares, c.addr)
	server.Serve(ctx)
}

func (c *grpcCommand) Usage() {
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
			panic(fmt.Errorf("Unsupported host format, expected host:port"))
		}
		dest := parts[0]
		port := "80"
		if len(parts) == 2 {
			port = parts[1]
		}
		c.addr = host.NewListener("tcp", dest, port)
	}
}
