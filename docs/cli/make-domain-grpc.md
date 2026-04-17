# make:domain:grpc

Adds a gRPC transport layer to an existing domain. Generates the proto definition, the gRPC service handler, and DI configuration.

## Usage

```sh
waffle make:domain:grpc <name>
```

Example:

```sh
waffle make:domain:grpc user
```

Run this after `waffle make:domain user`.

## Generated files

### Proto definition

**`api/<service>/proto/user/service.proto`**

A skeleton Protocol Buffer service definition:

```proto
syntax = "proto3";

package user;

option go_package = "mymodule/api/myservice/proto/user";

service UserService {
    rpc Get(GetRequest) returns (GetResponse);
}

message GetRequest {
    int64 id = 1;
}

message GetResponse {
    int64  id    = 1;
    string email = 2;
    string name  = 3;
}
```

Add your RPC methods here, then run `make generate` to compile the proto file into Go code.

### gRPC handler

**`domain/user/cmd/grpc/user/service.go`**

The gRPC service implementation. It satisfies the generated `UserServiceServer` interface and delegates to the domain service:

```go
package user

import (
    "context"
    proto "mymodule/api/myservice/proto/user"
    "mymodule/domain/user/service"
)

type UserGrpc struct {
    proto.UnimplementedUserServiceServer
    service *service.UserService
}

func NewUserGrpc(s *service.UserService) *UserGrpc {
    return &UserGrpc{service: s}
}

func (g *UserGrpc) Start(registrar grpc.ServiceRegistrar) {
    proto.RegisterUserServiceServer(registrar, g)
}

func (g *UserGrpc) Get(ctx context.Context, req *proto.GetRequest) (*proto.GetResponse, error) {
    user, err := g.service.Get(ctx, int(req.Id))
    if err != nil {
        return nil, err
    }
    return &proto.GetResponse{
        Id:    int64(user.Id),
        Email: user.Email,
        Name:  user.Name,
    }, nil
}
```

### DI configuration

Add the following inside `InitDi()` in `config/di/di.go`. Use `di.Define` (not `di.Wire[ServiceHandler]`) — gRPC handlers are concrete types collected by tag:

```go
import (
    grpcCommand "gitlab.com/gorib/waffle/tools/grpc"
    usergrpc "mymodule/domain/user/cmd/grpc/user"
)

// Inside InitDi():
di.Define(usergrpc.NewUserGrpc, di.Tag(grpcCommand.ServiceTag))
```

## Post-generation steps

```sh
make .tidy generate
```

This runs the proto compiler (if configured in the Makefile) and regenerates the DI container.

## Proto compilation

Add a `protoc` invocation to your `Makefile`:

```makefile
proto:
    protoc --go_out=. --go-grpc_out=. api/<service>/proto/user/service.proto
```

Or use `buf` if preferred. The generated Go files are committed alongside the proto source.

## Error mapping

To translate domain errors to gRPC status codes, implement `grpc.ErrorMapper` on the handler:

```go
import "google.golang.org/grpc/codes"

func (g *UserGrpc) Errors() map[error]codes.Code {
    return map[error]codes.Code{
        model.ErrUserNotFound: codes.NotFound,
        model.ErrUserConflict: codes.AlreadyExists,
    }
}
```

The gRPC tool collects all error maps from all registered handlers and applies them via middleware. See [tools/grpc.md](../tools/grpc.md) for details.
