.PHONY: deps build run dev test test-race test-e2e test-e2e-docker test-e2e-podman lint clean frontend-install frontend-build frontend-dev frontend-test

deps:
	go mod download
	go mod verify

frontend-install:
	cd internal/web/frontend && npm install

frontend-build: frontend-install
	cd internal/web/frontend && npm run build

frontend-dev:
	cd internal/web/frontend && npm run dev

frontend-test:
	cd internal/web/frontend && npm test

build: frontend-build
	go build -o bin/devagent .

run:
	go run .

dev: frontend-build
	go run . --config-dir=./config

test:
	go test ./...

test-race:
	go test -race ./...

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
