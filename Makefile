.PHONY: lint
lint:
	go vet ./...

.PHONY: test
test:
	go test -v -race -p 1 ./...
