.PHONY: test run fmt lint bench cover

test:
	go test -race ./...

run:
	go run ./cmd/demo

fmt:
	go fmt ./...

lint:
	golangci-lint run

bench:
	go test ./internal/service -run '^$$' -bench . -benchmem

cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out
