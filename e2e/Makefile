# e2e test suite for ndc-elasticsearch.
#
# Everything is driven by env vars so the same targets work locally and in CI:
#   UPDATE_GOLDEN=1   (re)generate golden.ddn.json / golden.es.json instead of comparing
#   FAIL_FAST=1       stop at the first failure (go test -failfast); ON in CI, OFF locally
#   KEEP_STACK=1      leave a case's docker compose stack up after it finishes (debugging)
#   E2E_CASE=<name>   restrict to a single case (see `e2e-case`)
#   STACK_VERSION=..  pin the Elasticsearch/Kibana/connector stack (default 8.13.4)
#   ANTHROPIC_API_KEY the LLM key used for result comparison (required unless UPDATE_GOLDEN=1)
#
# The suite is gated behind the `e2e` build tag AND E2E=1 so the fast unit CI is
# never affected.

REPO_ROOT := $(abspath $(dir $(lastword $(MAKEFILE_LIST)))/..)
GOTEST    := E2E=1 go test -tags e2e -count=1 -timeout 90m -v ./e2e/harness/...

# Add -failfast when FAIL_FAST=1.
ifeq ($(FAIL_FAST),1)
GOTEST += -failfast
endif

.PHONY: e2e e2e-case e2e-report e2e-update-golden e2e-clean e2e-preflight help

## e2e: run the full suite (all cases discovered under e2e/cases)
e2e: e2e-preflight
	cd $(REPO_ROOT) && $(GOTEST)

## e2e-case: run a single case, e.g. `make e2e-case CASE=custom_products`
e2e-case: e2e-preflight
ifndef CASE
	$(error CASE is required, e.g. `make e2e-case CASE=custom_products`)
endif
	cd $(REPO_ROOT) && E2E_CASE=$(CASE) $(GOTEST) -run 'TestE2E/$(CASE)'

## e2e-update-golden: (re)generate goldens for all cases (or one, via CASE=)
# Recurse so UPDATE_GOLDEN=1 is exported to the whole run (including preflight,
# which then does not require ANTHROPIC_API_KEY).
e2e-update-golden:
	$(MAKE) $(if $(CASE),e2e-case CASE=$(CASE),e2e) UPDATE_GOLDEN=1

## e2e-report: print the location of the last report and show its markdown summary
e2e-report:
	@echo "Report artifacts:"
	@echo "  $(REPO_ROOT)/e2e/report/e2e-report.json"
	@echo "  $(REPO_ROOT)/e2e/report/e2e-report.md"
	@echo
	@cat $(REPO_ROOT)/e2e/report/e2e-report.md 2>/dev/null || echo "(no report yet; run `make e2e` first)"

## e2e-clean: remove report artifacts and any leftover e2e docker compose stacks
e2e-clean:
	@rm -rf $(REPO_ROOT)/e2e/report
	@echo "Removing leftover e2e-* compose projects (if any)..."
	@for p in $$(docker compose ls -a --format json 2>/dev/null | tr ',' '\n' | grep -o '"Name":"e2e-[^"]*"' | cut -d'"' -f4 | sort -u); do \
		echo "  down $$p"; docker compose -p $$p down -v --remove-orphans 2>/dev/null || true; \
	done

## e2e-preflight: verify required tooling is available
e2e-preflight:
	@command -v docker >/dev/null 2>&1 || { echo "ERROR: docker is required"; exit 1; }
	@docker compose version >/dev/null 2>&1 || { echo "ERROR: 'docker compose' plugin is required"; exit 1; }
	@command -v ddn >/dev/null 2>&1 || { echo "ERROR: the Hasura 'ddn' CLI is required (see e2e/README.md)"; exit 1; }
	@command -v go >/dev/null 2>&1 || { echo "ERROR: go is required"; exit 1; }
	@if [ -z "$$UPDATE_GOLDEN" ] && [ -z "$$ANTHROPIC_API_KEY" ]; then \
		echo "ERROR: ANTHROPIC_API_KEY is required for LLM comparison (or run with UPDATE_GOLDEN=1)"; exit 1; \
	fi

help:
	@grep -E '^## ' $(lastword $(MAKEFILE_LIST)) | sed 's/## //'
