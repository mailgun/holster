.PHONY: build
build:
	go build -v -o jobs-cli ./cmd/jobs-cli

lint:
	go vet ./...
