all: .tidy generate build

audit:
	@which golangci-lint >/dev/null || (echo "Cannot run linters. Have you installed golangci-lint?" && false)
	@golangci-lint run

.tidy:
	@go mod tidy

generate:
	@go generate ./...

# Separate from generate: mockgen reads the packages the container names, so
# it wants a tree that already compiles, and the mock set changes rarely.
mocks:
	@./waffle make:tests:mocks

build:
	@[ -d cmd ] && go build -ldflags "-w" -o bin/ ./cmd/...
