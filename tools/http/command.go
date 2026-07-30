package http

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/pborman/getopt/v2"

	"gitlab.com/gorib/http/openapi"
	"gitlab.com/gorib/http/server"
	"gitlab.com/gorib/pry"
	"gitlab.com/gorib/waffle/app"
)

type HttpCommand interface {
	app.Command
}

func NewHttpCommand(
	mux server.Mux,
	router server.Router,
	command app.BaseCommand,
	addr net.Addr,
	logger pry.Logger,
	opts ...server.Option,
) *httpCommand {
	return &httpCommand{
		BaseCommand: command,
		mux:         mux,
		router:      router,
		addr:        addr,
		logger:      logger,
		opts:        opts,
	}
}

type httpCommand struct {
	app.BaseCommand

	mux         server.Mux
	router      server.Router
	addr        net.Addr
	logger      pry.Logger
	opts        []server.Option
	openapiPath string

	server server.Server
}

func (c *httpCommand) Run(ctx context.Context) error {
	if err := c.Usage(ctx); err != nil {
		return err
	}
	if c.openapiPath != "" {
		c.router.Setup(c.mux)
		warns, err := openapi.Reconcile(c.mux.Routes(), c.openapiPath, c.Name())
		for _, w := range warns {
			_, _ = fmt.Fprintln(os.Stderr, "openapi:", w)
		}
		return err
	}
	c.server = server.NewServer(c.mux, c.router, c.addr, c.logger, c.opts...)
	return c.server.Serve(ctx)
}

func (c *httpCommand) Usage(ctx context.Context) error {
	opts := getopt.New()
	var help bool
	opts.FlagLong(&help, "help", 'h', "Display this help message")
	listen := opts.StringLong("listen", 'l', "", "Listen for incoming connections on [host:port]", "host:port")
	docs := opts.StringLong("openapi", 0, "", "Generate the OpenAPI spec to <path> and exit (does not serve)", "path")
	if err := opts.Getopt(app.Args(ctx), nil); err != nil {
		return err
	}
	c.openapiPath = *docs
	if help {
		opts.SetParameters("")
		_, _ = os.Stderr.WriteString(c.Description())
		opts.PrintUsage(os.Stderr)
		return app.ErrUsage
	}
	if *listen != "" {
		parts := strings.Split(*listen, ":")
		if len(parts) > 2 {
			return fmt.Errorf("unsupported host format, expected host:port")
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
				return fmt.Errorf("invalid port number")
			}
		}
		c.addr, err = net.ResolveTCPAddr("tcp", net.JoinHostPort(dest, strconv.Itoa(int(port))))
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *httpCommand) Daemon() {}

func (c *httpCommand) Close() error {
	if c.server != nil {
		return c.server.Close()
	}
	return nil
}
