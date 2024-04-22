GOLANGCI_LINT = $(GOPATH)/bin/golangci-lint
GOLANGCI_LINT_VERSION = v1.57.2

.PHONY: lint
lint: $(GOLANGCI_LINT)
	$(GOLANGCI_LINT) run

$(GOLANGCI_LINT):
	curl -sfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(GOPATH)/bin $(GOLANGCI_LINT_VERSION)

.PHONY: test
test:
	go test -v -race -p 1 -trimpath -tags holster_test_mode ./...
