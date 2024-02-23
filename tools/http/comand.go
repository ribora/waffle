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
	Serve(ctx context.Context) error
	Close() error
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
	connection Server
}

func (c *httpCommand) Run(ctx context.Context) error {
	err := c.Usage()
	if err != nil {
		return err
	}

	c.connection = http.NewServer(c.router, c.addr)
	return c.connection.Serve(ctx)
}

func (c *httpCommand) Usage() error {
	var help bool
	getopt.FlagLong(&help, "help", 'h', "Display this help message")
	listen := getopt.StringLong("listen", 'l', "", "Listen for incoming connections on [host:port]", "host:port")
	getopt.Parse()
	if help {
		getopt.SetParameters("")
		_, _ = os.Stderr.WriteString(c.Description())
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

func (c *httpCommand) Close() error {
	if c.connection != nil {
		_ = c.connection.Close()
	}
	return nil
}
