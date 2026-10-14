.PHONY: all audit .tidy generate test replacements build run

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
	@./waffle surface
	@./waffle agents -q

# Fails when a test replaces something waffle.yaml does not allow, and says what
# it found. The same comparison runs inside go test; this target is the report.
# Without `check` the command lists what the containers expose instead.
replacements:
	@./waffle tests:replacements check

build:
	@[ -d cmd ] && go build -ldflags "-w" -o bin/ ./cmd/...

run: .tidy generate
	@go run ./cmd/app $(ARGS)
