.PHONY: deps build run dev test test-e2e test-e2e-docker test-e2e-podman lint clean

deps:
	go mod download
	go mod verify

build:
	go build -o bin/devagent .

run:
	go run .

dev:
	go run . --config-dir=./config

test:
	go test ./...

test-e2e:
	go test -tags=e2e -v -timeout=10m ./internal/e2e/...

test-e2e-docker:
	go test -tags=e2e -v -timeout=10m -run 'Docker' ./internal/e2e/...

test-e2e-podman:
	go test -tags=e2e -v -timeout=10m -run 'Podman' ./internal/e2e/...

lint:
	golangci-lint run

clean:
	rm -rf bin/
	go clean
