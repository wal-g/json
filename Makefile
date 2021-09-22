.PHONY: fmt lint test

fmt:
	go fmt ./...

lint:
	docker run --rm -v $(PWD):/app -w /app golangci/golangci-lint:v1.42.1 golangci-lint run --allow-parallel-runners ./... 

test:
	go test ./...
