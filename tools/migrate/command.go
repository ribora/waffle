package migrate

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/pborman/getopt/v2"
	"github.com/pressly/goose/v3"

	"gitlab.com/gorib/storage/sql"
	"gitlab.com/gorib/try"
	"gitlab.com/gorib/waffle/app"
)

type MigrateCommand interface {
	app.Command
}

func NewMigrateCommand(command app.BaseCommand, connection sql.Db, migrations fs.FS) *migrateCommand {
	return &migrateCommand{
		BaseCommand: command,
		connection:  connection,
		migrations:  migrations,
	}
}

type migrateCommand struct {
	app.BaseCommand
	connection sql.Db
	migrations fs.FS
}

func (c *migrateCommand) Run(context.Context) {
	var help bool
	getopt.FlagLong(&help, "help", 'h', "Display this help message")
	connection := getopt.StringLong("connection", 'c', "", "Database connection string specified for database type", "driver://user:pass@host:port")
	getopt.Parse()
	args := getopt.Args()
	if len(args) == 0 {
		args = append(args, "up")
	}
	if help {
		getopt.SetParameters("command")
		getopt.PrintUsage(os.Stderr)
		_, _ = fmt.Fprintf(os.Stderr, "Commands:\n")
		_, _ = fmt.Fprintf(os.Stderr, "\tup\t\tMigrate the DB to the most recent version available\n")
		_, _ = fmt.Fprintf(os.Stderr, "\tup-by-one\tMigrate the DB up by 1\n")
		_, _ = fmt.Fprintf(os.Stderr, "\tup-to VERSION\tMigrate the DB to a specific VERSION\n")
		_, _ = fmt.Fprintf(os.Stderr, "\tdown\t\tRoll back the version by 1\n")
		_, _ = fmt.Fprintf(os.Stderr, "\tdown-to VERSION\tRoll back to a specific VERSION\n")
		_, _ = fmt.Fprintf(os.Stderr, "\tredo\t\tRe-run the latest migration\n")
		_, _ = fmt.Fprintf(os.Stderr, "\treset\t\tRoll back all migrations\n")
		_, _ = fmt.Fprintf(os.Stderr, "\tstatus\t\tDump the migration status for the current DB\n")
		_, _ = fmt.Fprintf(os.Stderr, "\tversion\t\tPrint the current version of the database\n")
		os.Exit(2)
	}

	if *connection != "" {
		dsn := strings.Split(*connection, "://")
		if dsn[0] == "" {
			panic(fmt.Errorf("Invalid database connection string format"))
		}
		c.connection = sql.NewDb(dsn[0], *connection)
	}

	goose.SetBaseFS(c.migrations)
	try.ThrowError(goose.SetDialect(c.connection.Connect().DriverName()))
	goose.SetTableName("migrations")
	try.ThrowError(goose.Run(args[0], c.connection.Connect().DB, "migrations", args[1:]...))
}
