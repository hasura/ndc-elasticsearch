//go:build e2e

package harness

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Two schema docs differing only in collection/object-type order must
// canonicalize to identical bytes (the connector emits them in nondeterministic
// Go-map order).
func TestCanonicalizeSchema_DeterministicAcrossOrder(t *testing.T) {
	a := []byte(`{
		"collections": [{"name":"b","type":"b"},{"name":"a","type":"a"}],
		"object_types": {"z":{"fields":{}},"a":{"fields":{}}},
		"scalar_types": {},
		"functions": [],
		"procedures": []
	}`)
	b := []byte(`{
		"collections": [{"name":"a","type":"a"},{"name":"b","type":"b"}],
		"object_types": {"a":{"fields":{}},"z":{"fields":{}}},
		"scalar_types": {},
		"functions": [],
		"procedures": []
	}`)

	ca, err := canonicalizeSchema(a)
	require.NoError(t, err)
	cb, err := canonicalizeSchema(b)
	require.NoError(t, err)

	assert.Equal(t, string(ca), string(cb),
		"canonical schema must be independent of collection / object-type ordering")

	// collections must be sorted by name.
	var doc map[string]interface{}
	require.NoError(t, json.Unmarshal(ca, &doc))
	cols := doc["collections"].([]interface{})
	require.Len(t, cols, 2)
	assert.Equal(t, "a", cols[0].(map[string]interface{})["name"])
	assert.Equal(t, "b", cols[1].(map[string]interface{})["name"])
}

// Canonicalizing the same input repeatedly must yield identical bytes.
func TestCanonicalizeSchema_StableAcrossRepeatedMarshal(t *testing.T) {
	in := []byte(`{
		"collections": [{"name":"c"},{"name":"a"},{"name":"b"}],
		"object_types": {"products.manufacturer":{"fields":{"name":{"type":{"type":"named","name":"keyword"}}}}},
		"scalar_types": {"keyword":{}},
		"functions": [],
		"procedures": []
	}`)
	first, err := canonicalizeSchema(in)
	require.NoError(t, err)
	for i := 0; i < 20; i++ {
		got, err := canonicalizeSchema(in)
		require.NoError(t, err)
		assert.Equalf(t, string(first), string(got), "canonical output must be stable (iteration %d differed)", i)
	}
}

func TestCanonicalizeSchema_InvalidJSON(t *testing.T) {
	_, err := canonicalizeSchema([]byte(`{ not valid json`))
	assert.Error(t, err)
}

func TestIsPendingGolden(t *testing.T) {
	assert.True(t, isPendingGolden([]byte(`{"__pending__": true}`)))
	assert.False(t, isPendingGolden([]byte(`{"__pending__": false}`)))
	assert.False(t, isPendingGolden([]byte(`{"collections": []}`)))
	assert.False(t, isPendingGolden([]byte(`not json at all`)))
}
