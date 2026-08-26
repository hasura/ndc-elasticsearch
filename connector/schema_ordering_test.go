package connector_test

import (
	"encoding/json"
	"sort"
	"testing"

	"github.com/hasura/ndc-elasticsearch/connector"
	"github.com/stretchr/testify/assert"
)

// collectionNames returns the collection names in the exact order they are
// emitted in the marshaled /schema response (i.e. slice order, not sorted by us
// afterwards).
func collectionNames(t *testing.T, raw []byte) []string {
	t.Helper()
	var m struct {
		Collections []struct {
			Name string `json:"name"`
		} `json:"collections"`
	}
	assert.NoError(t, json.Unmarshal(raw, &m))
	names := make([]string, 0, len(m.Collections))
	for _, c := range m.Collections {
		names = append(names, c.Name)
	}
	return names
}

// TestSchemaCollectionsOrderingDeterministic guards the customer-reported
// (JPMC) symptom that survived the object-type-collision fix: the top-level
// `collections` array is a JSON array, so encoding/json preserves its slice
// order. That order used to be the Go map-iteration order of the `indices` /
// `queries` maps, which is randomized per process — so restarting the connector
// against an identical configuration.json produced a byte-different /schema
// response (differing only in collection order).
//
// Go also randomizes map iteration order across separate `range` statements
// within a single process, so regenerating the schema many times in one test
// process exercises many iteration orders. The full marshaled response must now
// be byte-identical every time, and collections must be sorted by name.
func TestSchemaCollectionsOrderingDeterministic(t *testing.T) {
	// Several indices plus a native query, so both code paths that append to
	// Collections are covered. Names are intentionally not in sorted order in
	// the source literal.
	const cfgJSON = `{
	  "indices": {
	    "orders":   {"mappings": {"properties": {"id": {"type": "keyword"}}}},
	    "accounts": {"mappings": {"properties": {"id": {"type": "keyword"}}}},
	    "products": {"mappings": {"properties": {"id": {"type": "keyword"}}}},
	    "customers":{"mappings": {"properties": {"id": {"type": "keyword"}}}},
	    "invoices": {"mappings": {"properties": {"id": {"type": "keyword"}}}}
	  },
	  "queries": {
	    "zeta_query":  {"index": "orders",   "dsl": {"internal": {}}, "return_type": {"kind": "index"}},
	    "alpha_query": {"index": "accounts", "dsl": {"internal": {}}, "return_type": {"kind": "index"}}
	  }
	}`

	want := []string{
		"accounts", "alpha_query", "customers", "invoices",
		"orders", "products", "zeta_query",
	}

	var first []byte
	for i := 0; i < 50; i++ {
		st := newCollisionState(t, cfgJSON)
		resp := connector.ParseConfigurationToSchema(st.Configuration, st)
		raw, err := json.Marshal(resp)
		assert.NoError(t, err)

		// (a) Whole /schema response is byte-identical across regenerations.
		if i == 0 {
			first = raw
		} else {
			assert.Equalf(t, string(first), string(raw),
				"whole /schema response must be byte-deterministic (run %d differed)", i)
		}

		// (b) Collections are emitted in name-sorted order.
		got := collectionNames(t, raw)
		assert.Equalf(t, want, got, "collections must be sorted by name (run %d)", i)
		assert.Truef(t, sort.StringsAreSorted(got), "collection names must be sorted (run %d)", i)
	}
}
