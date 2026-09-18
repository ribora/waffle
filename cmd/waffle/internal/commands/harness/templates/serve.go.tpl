package harness

import (
{{if .GrpcHandlers}}waffleGrpc "gitlab.com/gorib/waffle/tools/grpc"
{{end}}{{if .HttpMux}}waffleHttp "gitlab.com/gorib/waffle/tools/http"
{{end}})

// One struct however many transports the service serves, so a scenario asks the
// harness for its caller once.
type Clients struct {
{{if .HttpMux}}// The base URL and an ordinary HTTP client.
*waffleHttp.InProcess
{{end}}{{if .GrpcHandlers}}// Add a typed client per handler: proto.NewXServiceClient(grpc.Conn()).
grpc *waffleGrpc.InProcess
{{end}}}

func (c *Clients) Close() {
{{if .HttpMux}}if c.InProcess != nil {
c.InProcess.Close()
}
{{end}}{{if .GrpcHandlers}}if c.grpc != nil {
c.grpc.Close()
}
{{end}}}

func (a *App) Serve() (*Clients, error) {
clients := &Clients{}
{{if .GrpcHandlers}}
// The container's own collections, not a copy of them: a handler or an
// interceptor added by di.Tag reaches the suite with no edit here, and the
// order is the daemon's.
grpc, err := waffleGrpc.Serve(
a.{{.GrpcHandlers}}(),
a.{{.GrpcInterceptors}}(),
{{if .GrpcStream}}a.{{.GrpcStream}}(),{{else}}nil,{{end}}
)
if err != nil {
return nil, err
}
clients.grpc = grpc
{{end}}{{if .HttpMux}}
// The collection, for the same reason: a router added by di.Tag reaches the
// suite with no edit here.
web, err := waffleHttp.Serve(a.{{.HttpMux}}(), a.{{.HttpRouters}}())
if err != nil {
return nil, err
}
clients.InProcess = web
{{end}}
return clients, nil
}
