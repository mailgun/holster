GOLINT = $(GOPATH)/bin/golangci-lint

$(GOLINT):
	curl -sfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(GOPATH)/bin v1.49.0

.PHONY: lint
lint: $(GOLINT)
	$(GOLINT) run

.PHONY: test
test:
	go test -v -race -p 1 ./...
