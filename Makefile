.PHONY: build fmt lint test

build:
	go build -o bin/agentctl ./cmd/agentctl

fmt:
	gofmt -w ./cmd ./internal

lint:
	go vet ./...

test:
	go test ./...
