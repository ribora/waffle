audit:
	@which golangci-lint >/dev/null || (echo "Cannot run linters. Have you installed golangci-lint?" && false)
	@golangci-lint run
