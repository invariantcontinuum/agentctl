.PHONY: build fmt fmt-check lint test

build:
	go build -o bin/agentctl ./cmd/agentctl
	go build -o bin/agentd ./cmd/agentd

fmt:
	gofmt -w ./cmd ./internal

fmt-check:
	test -z "$$(gofmt -l ./cmd ./internal)"

lint:
	go vet ./...

test:
	go test ./...
