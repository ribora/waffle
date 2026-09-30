package trace

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"gitlab.com/gorib/failure"
)

// Inbound records every call the suite makes to the service's own server.
func Inbound(conn grpc.ClientConnInterface) grpc.ClientConnInterface { return inbound{conn} }

type inbound struct{ grpc.ClientConnInterface }

func (c inbound) Invoke(ctx context.Context, method string, args, reply any, opts ...grpc.CallOption) error {
	began := seq.Add(1)
	err := c.ClientConnInterface.Invoke(ctx, method, args, reply, opts...)
	call(began, "in", method, args, reply, err)
	return err
}

// The generated NewXClient over it hands each call to client by method name.
func Outbound(client any) grpc.ClientConnInterface { return outbound{reflect.ValueOf(client)} }

type outbound struct{ target reflect.Value }

func (c outbound) Invoke(ctx context.Context, method string, args, reply any, _ ...grpc.CallOption) error {
	fn := c.target.MethodByName(method[strings.LastIndex(method, "/")+1:])
	if !fn.IsValid() {
		return status.Errorf(codes.Unimplemented, "trace: %s has no method for %s", c.target.Type(), method)
	}
	results := fn.Call([]reflect.Value{reflect.ValueOf(ctx), reflect.ValueOf(args)})
	err, _ := results[1].Interface().(error)
	if err == nil && !results[0].IsNil() {
		proto.Merge(reply.(proto.Message), results[0].Interface().(proto.Message))
	}
	call(0, "out", method, args, reply, err)
	return err
}

func (c outbound) NewStream(context.Context, *grpc.StreamDesc, string, ...grpc.CallOption) (grpc.ClientStream, error) {
	return nil, status.Error(codes.Unimplemented, "trace: streams are not traced")
}

func call(began int64, dir, method string, args, reply any, err error) {
	if !on.Load() {
		return
	}
	r := Record{Seq: began, Dir: dir, Kind: "grpc", Name: method, Request: message(args)}
	if err != nil {
		r.Error = refusal(err)
	} else {
		r.Response = message(reply)
	}
	write(r)
}

func refusal(err error) string {
	var parts []string
	st, isStatus := status.FromError(err)
	if isStatus {
		parts = append(parts, st.Code().String())
	}
	if fe, ok := failure.FromError(err); ok && fe.Type() != "" {
		return strings.Join(append(parts, fe.Type()), " ")
	}
	if isStatus {
		for _, d := range st.Details() {
			if info, ok := d.(*errdetails.ErrorInfo); ok && info.Domain == failure.Kind {
				return strings.Join(append(parts, strings.ToLower(info.Reason)), " ")
			}
		}
		return strings.Join(parts, " ")
	}
	return "failed"
}

func message(v any) json.RawMessage {
	if m, ok := v.(proto.Message); ok {
		if body, err := protojson.Marshal(m); err == nil {
			return body
		}
	}
	body, _ := json.Marshal(v)
	return body
}
