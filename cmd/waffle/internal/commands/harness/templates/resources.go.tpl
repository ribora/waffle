// The component harness. What is shared lives in
// gorib/waffle/docs/conventions/component-harness.md.
package harness

import (
"context"
{{if .Migrations}}dbSql "database/sql"
"embed"
{{end}}"os"

{{if .Migrations}}"github.com/pressly/goose/v3"
{{end}}{{if .Redis}}"gitlab.com/gorib/redis/redistest"
{{end}}{{if .Postgres}}"gitlab.com/gorib/sql/sqltest"
{{end}}"gitlab.com/gorib/waffle/resource"
{{range .Migrations}}{{.Alias}} "{{.Path}}"
{{end}})
{{if .Postgres}}
// Without {{.Env}}_TEST_DSN a container is started and the next run reuses it.
// A database per process, dropped in Down.
var postgres = sqltest.New(
sqltest.Env("{{.Env}}_TEST_DSN"),
sqltest.Docker("postgres:18-alpine"),
sqltest.Database("{{.Service}}_test"),
sqltest.Ephemeral(),
{{if .Migrations}}sqltest.Migrations(migrations),
{{end}})
{{end}}{{if .Redis}}
// Without {{.Env}}_TEST_REDIS a container of this run's own is started, and
// Down removes it: starting one costs less than keeping two suites out of each
// other's keyspace.
var cache = redistest.New(
redistest.Env("{{.Env}}_TEST_REDIS"),
redistest.Docker("redis:8-alpine"),
)
{{end}}
// The registry. Adding a system is adding it here, and nothing else changes.
var resources = resource.New({{range .Resources}}{{.}}, {{end}})
{{if .Migrations}}
func migrations(ctx context.Context, handle *dbSql.DB) error {
for _, source := range []embed.FS{
{{range .Migrations}}{{.Alias}}.NewMigrations(),
{{end}}} {
goose.SetBaseFS(source)
goose.SetTableName("migrations")
goose.SetLogger(goose.NopLogger())
if err := goose.UpContext(ctx, handle, "migrations", goose.WithAllowMissing()); err != nil {
return err
}
}
return nil
}
{{end}}
func Up(ctx context.Context) error { return resources.Up(ctx) }

func Reset(ctx context.Context) error { return resources.Reset(ctx) }

// From TestMain only: whatever is up is shared by every test in the package.
func Down(ctx context.Context) error { return resources.Down(ctx) }

// Nothing listens in a test, but the seam carrying the address is not replaced,
// so the values have to be there.
func Env() {
for key, value := range map[string]string{
"DAEMON_HOST": "127.0.0.1",
"DAEMON_PORT": "0",
"SERVICE":     "{{.Service}}-test",
"ENVIRONMENT": "test",
} {
if _, set := os.LookupEnv(key); !set {
_ = os.Setenv(key, value)
}
}
}
