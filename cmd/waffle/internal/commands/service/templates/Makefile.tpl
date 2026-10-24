.PHONY: all audit .tidy generate test build run

all: .tidy generate build

audit:
	@which golangci-lint >/dev/null || (echo "Cannot run linters. Have you installed golangci-lint?" && false)
	@golangci-lint run

# Not in `all`: the component suite boots the container over real
# infrastructure, and `all` runs after every edit. Before a commit, and in CI.
test:
	@go test ./...

.tidy:
	@go mod tidy

generate:
	@go generate ./...
	@./waffle agents -q

build:
	@[ -d cmd ] && go build -ldflags "-w" -o bin/ ./cmd/...

run: .tidy generate
	@go run ./cmd/app $(ARGS)
