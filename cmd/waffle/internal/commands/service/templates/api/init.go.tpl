package {{ .Service }}

//go:generate mkdir -p pb
//go:generate find . -name *.proto -exec protoc -I proto --go_out=pb --go_opt=paths=source_relative --go-grpc_out=pb --go-grpc_opt=paths=source_relative {} +
