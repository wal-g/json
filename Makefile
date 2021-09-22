.PHONY: fmt lint test

fmt:
	go fmt ./...

test:
	go test ./...
