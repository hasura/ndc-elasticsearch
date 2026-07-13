.PHONY: format
format:
	gofmt -w -s .

.PHONY: unit-test
unit-test:
	go test -v -race -timeout 3m ./...

.PHONY: test
test: unit-test

# End-to-end suite (gated behind the `e2e` build tag + E2E=1; unaffected by the
# fast unit CI above). See e2e/README.md. These just delegate to e2e/Makefile.
.PHONY: e2e e2e-case
e2e:
	$(MAKE) -C e2e e2e

e2e-case:
	$(MAKE) -C e2e e2e-case CASE=$(CASE)

# Install golangci-lint tool to run lint locally
# https://golangci-lint.run/usage/install
.PHONY: lint
lint:
	golangci-lint run

.PHONY: generate
generate:
	$(SHELL) ./scripts/generate.sh

.PHONY: update-deps
update-deps:
	go get -u ./...
	go mod tidy