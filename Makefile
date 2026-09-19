.PHONY: test audit docs
.IGNORE: audit

all: audit test

test:
	@go test ./...

docs:
	@go test ./cmd/waffle/ -run 'TestDocs|TestReadme|TestRunningLocally|TestEveryPage'

audit:
	@which golangci-lint >/dev/null || (echo "Cannot run linters. Have you installed golangci-lint?" && false)
	@golangci-lint run
