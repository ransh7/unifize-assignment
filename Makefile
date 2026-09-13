.PHONY: test run fmt lint

test:
	go test -race ./...

run:
	go run ./cmd/demo

fmt:
	go fmt ./...

lint:
	golangci-lint run
