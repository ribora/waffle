# gRPC Handler Pattern

[Docs](../README.md) · Tools

This page describes how to implement gRPC domain handlers in a waffle service.

## Handler struct

Each domain that exposes gRPC has a handler struct in the flat `grpc` package (`domain/<x>/cmd/grpc/service.go`, no per-domain subpackage) that:
1. Embeds the bare protoc-generated `proto.XxxServiceServer` interface — a deliberate fail-loud choice, **not** `UnimplementedXxxServiceServer`: calling an RPC you haven't implemented panics immediately instead of silently returning `codes.Unimplemented`.
2. Holds a consumer-defined management interface as its only dependency.

```go
package grpc

import (
    "context"

    proto "mymodule/api/pb/user"
    "mymodule/domain/user/model"
    "github.com/samber/lo"
    "google.golang.org/grpc"
)

// UserManagement is defined here, in the consumer (cmd/grpc), not in the service layer.
type UserManagement interface {
    GetUser(ctx context.Context, id int) (*model.User, error)
    CreateUser(ctx context.Context, email, name string) (*model.User, error)
    ListUsers(ctx context.Context) ([]*model.User, error)
}

func NewUserService(userManagement UserManagement) *userService {
    return &userService{userManagement: userManagement}
}

type userService struct {
    proto.UserServiceServer // bare interface: unimplemented RPC = panic (fail-loud)
    userManagement UserManagement
}
```

The proto import path comes from `api_import` in `waffle.yaml` (`<api_import>/<domain>`).

## Registering with the gRPC server

Implement `Start` to register with the gRPC server registrar:

```go
func (s *userService) Start(server grpc.ServiceRegistrar) {
    proto.RegisterUserServiceServer(server, s)
}
```

## Implementing RPC methods

Each method receives a proto request, calls the management interface, and returns a proto response. Domain errors propagate as-is — the error mapper middleware translates them to gRPC status codes. These examples call the hand-written helpers of `mapper: dto`, below; under the default `mapper: goverter` the same calls go to the generated `convert` package.

```go
func (s *userService) Get(ctx context.Context, req *proto.GetUserRequest) (*proto.User, error) {
    user, err := s.userManagement.GetUser(ctx, int(req.Id))
    if err != nil {
        return nil, err
    }
    return userToProto(user), nil
}

func (s *userService) Create(ctx context.Context, req *proto.CreateUserRequest) (*proto.User, error) {
    user, err := s.userManagement.CreateUser(ctx, req.Email, req.Name)
    if err != nil {
        return nil, err
    }
    return userToProto(user), nil
}

func (s *userService) List(ctx context.Context, _ *proto.ListUsersRequest) (*proto.UserList, error) {
    users, err := s.userManagement.ListUsers(ctx)
    if err != nil {
        return nil, err
    }
    return &proto.UserList{Items: lo.Map(users, userToProtoList)}, nil
}
```

## dto.go — proto ↔ domain conversions

With the default `mapper: goverter` conversions live in a generated `cmd/grpc/convert` package and there is no `dto.go` — see [dto-conversion.md](../conventions/dto-conversion.md).

With `mapper: dto` in `waffle.yaml` nothing extra is scaffolded — write conversions by hand where you need them: trivially inline in the handler, or in a `dto.go` file you add to the same package. Conversion functions are pure — no logic, no errors:

```go
// cmd/grpc/dto.go
package grpc

import (
    proto "mymodule/api/pb/user"
    "mymodule/domain/user/model"
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

Hand-written conversions are opt-in for a reason: they compile whatever they leave out, so a field added to `model.User` silently stops reaching the wire. goverter refuses to generate instead.

## Error mapping

Error mapping is done by a dedicated `errorHandler` (scaffolded in `cmd/grpc/errors.go`, registered under `grpcCommand.ErrorMapperTag`), not by the handler itself. See [tools/grpc.md](grpc.md#error-mapping) for the `ErrorMapper` pattern.

The handler just returns domain errors directly:

```go
if err != nil {
    return nil, err  // translated to gRPC status code by the error mapper middleware
}
```

## Wiring in DI

`make:domain:grpc` generates `config/di/di_user_grpc.go`. The handler and the error mapper are concrete types collected by tag — use `di.Define`. The handler's `UserManagement` interface is wired from the handle `config/di/di_user_domain.go` holds:

```go
import userGrpc "mymodule/domain/user/cmd/grpc"

di.Define(userGrpc.NewUserService, di.Tag(grpcCommand.ServiceTag))
di.Define(userGrpc.NewErrorHandler, di.Tag(grpcCommand.ErrorMapperTag))

di.Wire[userGrpc.UserManagement](userManagementInstance)
```

## File layout

```
domain/user/cmd/grpc/
├── service.go        Handler struct, Start(), RPC methods
├── convert/          Generated goverter converter (default, mapper: goverter)
├── dto.go            Proto ↔ domain conversions — hand-added when mapper: dto
└── errors.go         ErrorMapper (error → codes.Code map for this domain)
```

The package is flat `grpc` — one handler per domain, no per-domain subpackage. All registered error maps are merged by the gRPC tool at startup.
