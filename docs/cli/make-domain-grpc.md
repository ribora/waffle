# make:domain:grpc

Adds a gRPC transport layer to an existing domain. Generates the proto definition, the gRPC service handler, an error mapper stub, and DI configuration.

## Usage

```sh
waffle make:domain:grpc <name>
```

Example:

```sh
waffle make:domain:grpc user
```

Run this after `waffle make:domain user`.

## waffle.yaml

Behavior is driven by the project's `waffle.yaml` (see [overview](overview.md#waffle.yaml)):

| Key | Used for |
|-----|----------|
| `api_import` | Import path prefix for generated proto stubs (`{{ api_import }}/<domain>`) |
| `proto_dir` | Where the `.proto` file is written (`<proto_dir>/<domain>/service.proto`) |
| `mapper` | `"goverter"` (the default) scaffolds a `convert/convert.go`; `"dto"` scaffolds nothing extra (write conversions by hand) |

A missing `waffle.yaml` behaves exactly like one containing only the defaults.

## Generated files

### Proto definition

**`<proto_dir>/<domain>/service.proto`**

A skeleton Protocol Buffer service definition:

```proto
syntax = "proto3";

package user;
option go_package = "../pb/user";

service UserService {
}
```

Add your RPC methods here, then run `make generate` to compile the proto file into Go code.

### gRPC handler

**`domain/user/cmd/grpc/service.go`**

The package is flat `grpc` (no per-domain subpackage). It embeds the generated `proto.UserServiceServer` interface directly — a deliberate fail-loud choice, not `UnimplementedUserServiceServer`: an unimplemented RPC panics instead of silently returning `codes.Unimplemented`, so a forgotten method surfaces immediately instead of at a caller's runtime.

```go
package grpc

import (
    proto "mymodule/api/pb/user"
    "google.golang.org/grpc"
)

type UserManagement interface {
}

func NewUserService(userManagement UserManagement) *userService {
    return &userService{userManagement: userManagement}
}

type userService struct {
    proto.UserServiceServer
    userManagement UserManagement
}

func (s *userService) Start(server grpc.ServiceRegistrar) {
    proto.RegisterUserServiceServer(server, s)
}
```

Add your RPC methods to `userService`, declare what they need on `UserManagement`, and implement that interface on the domain service.

### DTO conversion

**`domain/user/cmd/grpc/convert/convert.go`** — scaffolded with `mapper: "goverter"`, the default (the goverter tool is added to `go.mod` if missing). See [dto-conversion.md](../conventions/dto-conversion.md).

With `mapper: "dto"` nothing is generated for conversions — write them by hand in the `grpc` package (inline in `service.go` or in a `dto.go` you add), and accept that a field added to the model will not fail anything when the conversion forgets it.

### Errors

**`domain/user/cmd/grpc/errors.go`**

An empty error mapper, registered under `grpcCommand.ErrorMapperTag`:

```go
package grpc

import (
    "google.golang.org/grpc/codes"
)

func NewErrorHandler() *errorHandler {
    return &errorHandler{}
}

type errorHandler struct{}

func (h *errorHandler) Errors() map[error]codes.Code {
    return map[error]codes.Code{}
}
```

Fill in the map as domain errors need specific gRPC status codes:

```go
func (h *errorHandler) Errors() map[error]codes.Code {
    return map[error]codes.Code{
        model.ErrUserNotFound: codes.NotFound,
        model.ErrUserConflict: codes.AlreadyExists,
    }
}
```

### DI configuration

**`config/di/di_user_grpc.go`**

```go
package di

import (
    userGrpc "mymodule/domain/user/cmd/grpc"

    "gitlab.com/gorib/di"
    grpcCommand "gitlab.com/gorib/waffle/tools/grpc"
)

func InitUserGrpc() {
    di.Define(userGrpc.NewUserService, di.Tag(grpcCommand.ServiceTag))
    di.Define(userGrpc.NewErrorHandler, di.Tag(grpcCommand.ErrorMapperTag))

    di.Wire[userGrpc.UserManagement](userManagementInstance)
}
```

Both the service handler and the error mapper are concrete types collected by tag — use `di.Define`, not `di.Wire[ServiceHandler]`. Replace `userManagementInstance` with whatever provides `UserManagement` (typically the domain service, wired via `di.Alias[userGrpc.UserManagement]()`).

Call `InitUserGrpc()` from `InitDi()` in `config/di/di.go`.

## Post-generation steps

```sh
make .tidy generate
```

This regenerates the DI container.

## Proto compilation

When `proto_dir` resolves inside the current module (the default), `make:domain:grpc` runs `go generate ./api/...` itself before `make .tidy generate`, so `pb/user` exists locally and `go mod tidy` doesn't try to fetch it as a remote module.

When `proto_dir` points outside the module (a centralized proto repo shared by several services, for example), the command skips that step and prints a note — run the proto generator for that repo separately, then `make .tidy generate` here.

## Error mapping

See [Errors](#errors) above for the per-domain map, and [tools/grpc.md](../tools/grpc.md) for how the gRPC tool aggregates all registered `ErrorMapper`s across domains.
