package migrate

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"

	"github.com/pborman/getopt/v2"

	"gitlab.com/gorib/sql"
	"gitlab.com/gorib/waffle/app"
)

type migrationsTag struct{}

var MigrationsTag = migrationsTag{}

type MigrateCommand interface {
	app.Command
}

func NewMigrateCommand(migrations []fs.FS, command app.BaseCommand, connection sql.Db, driver string) *migrateCommand {
	return &migrateCommand{
		BaseCommand: command,
		connection:  connection,
		migrations:  migrations,
		driver:      driver,
	}
}

type migrateCommand struct {
	app.BaseCommand
	connection sql.Db
	migrations []fs.FS
	driver     string
}

func (c *migrateCommand) Run(ctx context.Context) (err error) {
	opts := getopt.New()
	var help bool
	opts.FlagLong(&help, "help", 'h', "Display this help message")
	connection := opts.StringLong("connection", 'c', "", "Database connection string specified for database type", "driver://user:pass@host:port")
	if err := opts.Getopt(app.Args(ctx), nil); err != nil {
		return err
	}
	args := opts.Args()
	if len(args) == 0 {
		args = append(args, "up")
	}
	if help {
		opts.SetParameters("command")
		opts.PrintUsage(os.Stderr)
		app.PrintList(os.Stderr, "Commands:", map[string]string{
			"up":              "Migrate the DB to the most recent version available",
			"up-by-one":       "Migrate the DB up by 1",
			"up-to VERSION":   "Migrate the DB to a specific VERSION",
			"down":            "Roll back the version by 1",
			"down-to VERSION": "Roll back to a specific VERSION",
			"redo":            "Re-run the latest migration",
			"reset":           "Roll back all migrations",
			"status":          "Dump the migration status for the current DB",
			"version":         "Print the current version of the database",
		})
		return app.ErrUsage
	}

	if *connection != "" {
		dsn := strings.Split(*connection, "://")
		if dsn[0] == "" {
			return fmt.Errorf("invalid database connection string format")
		}
		c.connection, err = sql.NewDb(dsn[0], *connection)
		if err != nil {
			return err
		}
		if closer, ok := c.connection.(io.Closer); ok {
			defer func() { _ = closer.Close() }()
		}
		// The scheme names the database this run targets, so it names the dialect
		// too — DB_DRIVER describes the connection that was just replaced.
		c.driver = dsn[0]
	}

	db, err := c.connection.DbSql()
	if err != nil {
		return err
	}
	if err = db.Ping(); err != nil {
		return fmt.Errorf("cannot connect to db: %w", err)
	}
	return run(ctx, db, c.migrations, c.driver, args[0], args[1:])
}
