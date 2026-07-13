// Package harness contains the end-to-end (e2e) test harness for the
// ndc-elasticsearch connector.
//
// All of the real harness code lives in files guarded by the `//go:build e2e`
// build constraint, so it is completely invisible to the default build and to
// the fast unit-test CI (`go build ./...`, `make unit-test`). This file carries
// no build tag purely so that the package is non-empty under the default build
// and `go build ./...` does not fail with "build constraints exclude all Go
// files".
//
// To run the suite, use the e2e build tag (or the e2e Makefile):
//
//	E2E=1 go test -tags e2e -v ./e2e/harness/...
//
// See e2e/README.md for the full workflow and for how to add a new test case.
package harness
