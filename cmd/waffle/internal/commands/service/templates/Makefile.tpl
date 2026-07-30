all: .tidy generate build

audit:
	@which golangci-lint >/dev/null || (echo "Cannot run linters. Have you installed golangci-lint?" && false)
	@golangci-lint run

.tidy:
	@go mod tidy

generate:
	@go generate ./...

build:
	@[ -d cmd ] && go build -ldflags "-w" -o bin/ ./cmd/...
