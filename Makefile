.PHONY: format
format:
	gofmt -w -s .

.PHONY: unit-test
unit-test:
	go test -v -race -timeout 3m ./...

.PHONY: test
test: unit-test

# Install golangci-lint tool to run lint locally
# https://golangci-lint.run/usage/install
.PHONY: lint
lint:
	golangci-lint run

.PHONY: generate
generate:
	$(SHELL) ./scripts/generate.sh