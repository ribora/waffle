package http

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/pborman/getopt/v2"

	http "gitlab.com/gorib/http/server"
	"gitlab.com/gorib/pry"
	"gitlab.com/gorib/waffle/app"
)

type HttpCommand interface {
	app.Command
}

type Server interface {
	Serve(ctx context.Context) error
	Close() error
}

func NewHttpCommand(router http.Router, command app.BaseCommand, addr net.Addr, logger pry.Logger, middlewares []http.Middleware) *httpCommand {
	return &httpCommand{
		router:      router,
		addr:        addr,
		BaseCommand: command,
		logger:      logger,
		middlewares: middlewares,
	}
}

type httpCommand struct {
	router http.Router
	addr   net.Addr
	app.BaseCommand
	connection  Server
	logger      pry.Logger
	middlewares []http.Middleware
}

func (c *httpCommand) Run(ctx context.Context) error {
	err := c.Usage()
	if err != nil {
		return err
	}

	c.connection = http.NewServer(c.router, c.addr, c.logger, c.middlewares...)
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

func (c *httpCommand) Close() error {
	if c.connection != nil {
		_ = c.connection.Close()
	}
	return nil
}
