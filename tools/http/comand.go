package http

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/pborman/getopt/v2"

	"gitlab.com/gorib/server/host"
	"gitlab.com/gorib/server/http"
	"gitlab.com/gorib/waffle/app"
)

type HttpCommand interface {
	app.Command
}

type Server interface {
	Serve(ctx context.Context)
}

func NewHttpCommand(router http.Router, command app.BaseCommand, addr net.Addr) *httpCommand {
	return &httpCommand{
		router:      router,
		addr:        addr,
		BaseCommand: command,
	}
}

type httpCommand struct {
	router http.Router
	addr   net.Addr
	app.BaseCommand
}

func (c *httpCommand) Run(ctx context.Context) {
	c.Usage()

	server := http.NewServer(c.router, c.addr)
	server.Serve(ctx)
}

func (c *httpCommand) Usage() {
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
