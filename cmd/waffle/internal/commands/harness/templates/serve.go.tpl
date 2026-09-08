package harness

import (
{{if .GrpcHandlers}}gribgrpc "gitlab.com/gorib/grpc/server"
waffleGrpc "gitlab.com/gorib/waffle/tools/grpc"
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
grpc, err := waffleGrpc.Serve(
[]waffleGrpc.ServiceHandler{
{{range .GrpcHandlers}}a.{{.}}(),
{{end}}},
[]gribgrpc.Interceptor{
{{range .GrpcInterceptors}}a.{{.}}(),
{{end}}},
{{if .GrpcStream}}[]gribgrpc.StreamInterceptor{
{{range .GrpcStream}}a.{{.}}(),
{{end}}},{{else}}nil,{{end}}
)
if err != nil {
return nil, err
}
clients.grpc = grpc
{{end}}{{if .HttpMux}}
clients.InProcess = waffleHttp.Serve(a.{{.HttpMux}}(), a.{{.HttpRouter}}())
{{end}}
return clients, nil
}
