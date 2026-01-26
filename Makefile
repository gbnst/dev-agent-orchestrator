.PHONY: deps build run test lint clean

deps:
	go mod download
	go mod verify

build:
	go build -o bin/devagent .

run:
	go run .

test:
	go test ./...

lint:
	golangci-lint run

clean:
	rm -rf bin/
	go clean
