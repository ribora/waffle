# gRPC Handler Pattern

This page describes how to implement gRPC domain handlers in a waffle service.

## Handler struct

Each domain that exposes gRPC has a handler struct that:
1. Embeds the protoc-generated `proto.XxxServer` interface for forward compatibility (unimplemented methods return `codes.Unimplemented` automatically).
2. Holds a consumer-defined management interface as its only dependency.

```go
package grpc

import (
    "context"

    proto "mymodule/api/user"
    "mymodule/domain/user/model"
    "google.golang.org/grpc"
)

// UserManagement is defined here, in the consumer (cmd/grpc), not in the service layer.
type UserManagement interface {
    GetUser(ctx context.Context, id int) (*model.User, error)
    CreateUser(ctx context.Context, email, name string) (*model.User, error)
    ListUsers(ctx context.Context) ([]*model.User, error)
}

func NewUserGrpc(management UserManagement) *userGrpc {
    return &userGrpc{management: management}
}

type userGrpc struct {
    proto.UnimplementedUserServiceServer // forward-compatible stub
    management UserManagement
}
```

## Registering with the gRPC server

Implement `Start` to register with the gRPC server registrar:

```go
func (g *userGrpc) Start(server grpc.ServiceRegistrar) {
    proto.RegisterUserServiceServer(server, g)
}
```

## Implementing RPC methods

Each method receives a proto request, calls the management interface, and returns a proto response. Domain errors propagate as-is — the error mapper middleware translates them to gRPC status codes.

```go
func (g *userGrpc) Get(ctx context.Context, req *proto.GetUserRequest) (*proto.User, error) {
    user, err := g.management.GetUser(ctx, int(req.Id))
    if err != nil {
        return nil, err
    }
    return userToProto(user), nil
}

func (g *userGrpc) Create(ctx context.Context, req *proto.CreateUserRequest) (*proto.User, error) {
    user, err := g.management.CreateUser(ctx, req.Email, req.Name)
    if err != nil {
        return nil, err
    }
    return userToProto(user), nil
}

func (g *userGrpc) List(ctx context.Context, _ *proto.ListUsersRequest) (*proto.UserList, error) {
    users, err := g.management.ListUsers(ctx)
    if err != nil {
        return nil, err
    }
    return &proto.UserList{Items: lo.Map(users, userToProtoList)}, nil
}
```

## dto.go — proto ↔ domain conversions

Keep all proto conversion functions in a dedicated `dto.go` file in the same package. Conversion functions are pure — no logic, no errors:

```go
// cmd/grpc/dto.go
package grpc

import (
    proto "mymodule/api/user"
    "mymodule/domain/user/model"
    "github.com/samber/lo"
)

func userToProto(u *model.User) *proto.User {
    return &proto.User{
        Id:    int64(u.Id),
        Email: u.Email,
        Name:  u.Name,
    }
}

func userToProtoList(u *model.User, _ int) *proto.User {
    return userToProto(u)
}
```

The `lo.Map` helper requires a two-argument function `(T, int) R` — hence the `userToProtoList` variant.

## Error mapping

Error mapping is done by a dedicated middleware, not by the handler itself. See [tools/grpc.md](grpc.md#error-mapping) for the `ErrorMapper` pattern.

The handler just returns domain errors directly:

```go
if err != nil {
    return nil, err  // translated to gRPC status code by the error mapper middleware
}
```

## Wiring in DI

Register the handler as a `ServiceHandler` with `grpcCommand.ServiceTag` using `di.Define`. Register the service with `di.Alias` so the handler's `UserManagement` interface resolves to it:

```go
di.Define(usergrpc.NewUserGrpc, di.Tag(grpcCommand.ServiceTag))

di.Define(service.NewUserService,
    di.Alias[usergrpc.UserManagement](),
    // other aliases...
)
```

## File layout

```
domain/user/cmd/grpc/
├── service_user.go   Handler struct, Start(), RPC methods
├── dto.go            Proto ↔ domain conversion functions
└── errors.go         ErrorMapper middleware (shared across all handlers)
```

A single `errors.go` with an `errorHandler` middleware covers all domains — all error maps are merged by the gRPC tool at startup.
