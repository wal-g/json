TOOLS_MOD_DIR = ./internal/tools

.PHONY: fmt lint test install-tools

fmt: install-tools
	go fmt ./...
	goimports -w  -local github.com/wal-g/json ./

lint: install-tools
	golangci-lint run --allow-parallel-runners ./...

test:
	go test ./...

install-tools:
	cd $(TOOLS_MOD_DIR) && go install golang.org/x/tools/cmd/goimports
	cd $(TOOLS_MOD_DIR) && go install github.com/golangci/golangci-lint/cmd/golangci-lint
