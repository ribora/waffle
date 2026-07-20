package {{ .Service }}

//go:generate find . -name *.proto -exec protoc -I proto {} --go_out=proto --go-grpc_out=proto ;
