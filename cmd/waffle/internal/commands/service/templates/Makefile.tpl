all: .tidy generate build

audit:
	@which golangci-lint >/dev/null || (echo "Cannot run linters. Have you installed golangci-lint?" && false)
	@golangci-lint run

.tidy:
	@go mod tidy

generate:
	@go generate ./...

# Fails when a test replaces something waffle.yaml does not allow. Separate
# from build: it reads the container, so it wants a tree that already compiles.
replacements:
	@./waffle check:tests:replacements

build:
	@[ -d cmd ] && go build -ldflags "-w" -o bin/ ./cmd/...
