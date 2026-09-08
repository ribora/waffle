package harness

import (
"context"

{{if .Dead}}grpcClient "gitlab.com/gorib/grpc/client"
{{end}}{{if .Logger}}"gitlab.com/gorib/pry"
{{end}}{{if .Consumer}}"gitlab.com/gorib/waffle/tools/scribus"
{{end}}
"{{.Container}}"
)

// App is the booted container plus the recorders the harness put into it. They
// are handed back rather than passed in: a Replacements literal in a scenario is
// what the allow-list check reads, and a recorder the harness installs is not a
// seam the scenario has to be granted.
type App struct {
*{{.Package}}.Application
{{if .Consumer}}
Consumer *scribus.ConsumerRecorder{{end}}{{if .Logger}}
Logger *pry.LogRecorder{{end}}
}

// Fills in what is a condition of booting rather than a choice the test makes.
func Boot(ctx context.Context, cut {{.Package}}.Replacements) (*App, error) {
// Up here rather than left to the caller: before it {{if .Postgres}}the handle is nil and the
// container falls through to a real database out of an environment nobody set{{else}}nothing the
// resources own is ready{{end}}.
if err := Up(ctx); err != nil {
return nil, err
}

{{if .Consumer}}consumer := scribus.Recorder()
{{end}}{{if .Logger}}// A recorder rather than pry.Noop(): silence is right only where nothing is
// asserted, and swapping it later means editing this line.
logger := pry.Recorder()
{{end}}{{if .Postgres}}cut.SqlDb = postgres.Db()
{{end}}{{if .Logger}}cut.PryLogger = logger
{{end}}{{if .Consumer}}cut.ScribusConsumer = consumer
{{end}}{{if .Dead}}
// Replacing a seam does not prune what feeds it, so the connection under a
// replaced typed client is built anyway. The name is only the panic's label —
// rename it to the service that answers, which the container cannot know.
{{range .Dead}}if cut.{{.Field}} == nil {
cut.{{.Field}} = grpcClient.Dead("{{.Name}}")
}
{{end}}{{end}}
application, err := {{.Package}}.InitAppWith(cut)
if err != nil {
return nil, err
}
return &App{Application: application{{if .Consumer}}, Consumer: consumer{{end}}{{if .Logger}}, Logger: logger{{end}}}, nil
}
