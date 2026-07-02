package connector_test

import (
	"encoding/json"
	"sort"
	"testing"

	"github.com/hasura/ndc-elasticsearch/connector"
	"github.com/hasura/ndc-elasticsearch/types"
	"github.com/hasura/ndc-sdk-go/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// collisionState builds a minimal State from a configuration JSON literal.
func collisionState(t *testing.T, configJSON string) (*types.Configuration, *types.State) {
	t.Helper()
	var cfg types.Configuration
	require.NoError(t, json.Unmarshal([]byte(configJSON), &cfg))
	st := &types.State{
		SupportedSortFields:      make(map[string]interface{}),
		SupportedAggregateFields: make(map[string]interface{}),
		SupportedFilterFields:    make(map[string]interface{}),
		NestedFields:             make(map[string]interface{}),
		Configuration:            &cfg,
	}
	return &cfg, st
}

// nestedTypeName resolves the underlying named type of a nested field on a
// collection object type, unwrapping array/nullable wrappers.
func nestedTypeName(t *testing.T, resp *schema.SchemaResponse, collectionType, field string) string {
	t.Helper()
	raw, err := json.Marshal(resp)
	require.NoError(t, err)
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &m))

	objectTypes := m["object_types"].(map[string]interface{})
	ot, ok := objectTypes[collectionType].(map[string]interface{})
	require.Truef(t, ok, "object type %q not found", collectionType)
	fields := ot["fields"].(map[string]interface{})
	f, ok := fields[field].(map[string]interface{})
	require.Truef(t, ok, "field %q not found on %q", field, collectionType)

	typ := f["type"].(map[string]interface{})
	for {
		switch typ["type"].(string) {
		case "named":
			return typ["name"].(string)
		case "array":
			typ = typ["element_type"].(map[string]interface{})
		case "nullable":
			typ = typ["underlying_type"].(map[string]interface{})
		default:
			t.Fatalf("unexpected type kind %v", typ["type"])
		}
	}
}

// objectFieldNames returns the set of field names for a given object type.
func objectFieldNames(t *testing.T, resp *schema.SchemaResponse, typeName string) map[string]bool {
	t.Helper()
	raw, err := json.Marshal(resp)
	require.NoError(t, err)
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &m))

	objectTypes := m["object_types"].(map[string]interface{})
	ot, ok := objectTypes[typeName].(map[string]interface{})
	require.Truef(t, ok, "object type %q not found", typeName)
	out := map[string]bool{}
	for name := range ot["fields"].(map[string]interface{}) {
		out[name] = true
	}
	return out
}

// TestSchemaCollision_CrossIndexDifferentShapes documents the bug where two
// indices each define a nested object with the same name but different fields.
// Without the fix, one definition silently overwrites the other and fields are
// dropped. With the fix, each collection must reference a distinct object type
// that retains all of its fields.
//
// Mapping:
//
//	orders.address  → {street, city, country, postal_code}
//	returns.address → {city, country}
func TestSchemaCollision_CrossIndexDifferentShapes(t *testing.T) {
	const cfgJSON = `{
	  "indices": {
	    "orders": {"mappings": {"properties": {
	      "order_id": {"type": "keyword"},
	      "address": {"type": "nested", "properties": {
	        "street":      {"type": "text"},
	        "city":        {"type": "keyword"},
	        "country":     {"type": "keyword"},
	        "postal_code": {"type": "keyword"}
	      }}
	    }}},
	    "returns": {"mappings": {"properties": {
	      "return_id": {"type": "keyword"},
	      "address": {"type": "nested", "properties": {
	        "city":    {"type": "keyword"},
	        "country": {"type": "keyword"}
	      }}
	    }}}
	  },
	  "queries": {}
	}`

	cfg, st := collisionState(t, cfgJSON)
	resp := connector.ParseConfigurationToSchema(cfg, st)

	ordersAddrType := nestedTypeName(t, resp, "orders", "address")
	returnsAddrType := nestedTypeName(t, resp, "returns", "address")

	// The two address types have different structures — they must be separate
	// object types; sharing one means fields will be dropped.
	assert.NotEqual(t, ordersAddrType, returnsAddrType,
		"colliding address types must be disambiguated into separate object types")

	// orders.address must retain all four fields.
	ordersFields := objectFieldNames(t, resp, ordersAddrType)
	assert.True(t, ordersFields["street"], "orders.address must keep field 'street'")
	assert.True(t, ordersFields["city"], "orders.address must keep field 'city'")
	assert.True(t, ordersFields["country"], "orders.address must keep field 'country'")
	assert.True(t, ordersFields["postal_code"], "orders.address must keep field 'postal_code'")

	// returns.address must retain its two fields.
	returnsFields := objectFieldNames(t, resp, returnsAddrType)
	assert.True(t, returnsFields["city"], "returns.address must keep field 'city'")
	assert.True(t, returnsFields["country"], "returns.address must keep field 'country'")
	assert.Len(t, returnsFields, 2, "returns.address must have exactly 2 fields")
}

// TestSchemaCollision_CrossIndexIdenticalShapes verifies the minimal-churn
// guarantee: two indices that share an identically-structured nested object
// (e.g. an index and its alias) must collapse to a single bare-named object
// type. The fix must not introduce needless renames in this case.
func TestSchemaCollision_CrossIndexIdenticalShapes(t *testing.T) {
	const cfgJSON = `{
	  "indices": {
	    "events": {"mappings": {"properties": {
	      "event_type": {"type": "keyword"},
	      "metadata": {"type": "nested", "properties": {
	        "key":   {"type": "keyword"},
	        "value": {"type": "text"}
	      }}
	    }}},
	    "events_archive": {"mappings": {"properties": {
	      "event_type": {"type": "keyword"},
	      "metadata": {"type": "nested", "properties": {
	        "key":   {"type": "keyword"},
	        "value": {"type": "text"}
	      }}
	    }}}
	  },
	  "queries": {}
	}`

	cfg, st := collisionState(t, cfgJSON)
	resp := connector.ParseConfigurationToSchema(cfg, st)

	eventsMetaType := nestedTypeName(t, resp, "events", "metadata")
	archiveMetaType := nestedTypeName(t, resp, "events_archive", "metadata")

	// Identical structures must share a single bare-named type — no needless rename.
	assert.Equal(t, "metadata", eventsMetaType, "events.metadata must use the bare name 'metadata'")
	assert.Equal(t, "metadata", archiveMetaType, "events_archive.metadata must use the bare name 'metadata'")

	fields := objectFieldNames(t, resp, "metadata")
	assert.True(t, fields["key"], "metadata must have field 'key'")
	assert.True(t, fields["value"], "metadata must have field 'value'")
}

// TestSchemaCollision_WithinIndex documents the within-index variant of the
// bug: two top-level nested objects in the same index each contain a nested
// child with the same field name but different structures. Without the fix,
// both parent objects point at the same (corrupted) child type.
//
// Mapping (single index: profiles):
//
//	billing.contact  → {name, email, phone}
//	shipping.contact → {name, email}
func TestSchemaCollision_WithinIndex(t *testing.T) {
	const cfgJSON = `{
	  "indices": {
	    "profiles": {"mappings": {"properties": {
	      "user_id": {"type": "keyword"},
	      "billing": {"type": "nested", "properties": {
	        "contact": {"type": "nested", "properties": {
	          "name":  {"type": "text"},
	          "email": {"type": "keyword"},
	          "phone": {"type": "keyword"}
	        }}
	      }},
	      "shipping": {"type": "nested", "properties": {
	        "contact": {"type": "nested", "properties": {
	          "name":  {"type": "text"},
	          "email": {"type": "keyword"}
	        }}
	      }}
	    }}}
	  },
	  "queries": {}
	}`

	cfg, st := collisionState(t, cfgJSON)
	resp := connector.ParseConfigurationToSchema(cfg, st)

	billingContactType := nestedTypeName(t, resp, "billing", "contact")
	shippingContactType := nestedTypeName(t, resp, "shipping", "contact")

	// billing and shipping have structurally different contact children — they
	// must not share a single object type.
	assert.NotEqual(t, billingContactType, shippingContactType,
		"billing.contact and shipping.contact have different shapes and must be separate object types")

	// billing.contact must retain all three fields.
	billingFields := objectFieldNames(t, resp, billingContactType)
	assert.True(t, billingFields["name"], "billing.contact must keep field 'name'")
	assert.True(t, billingFields["email"], "billing.contact must keep field 'email'")
	assert.True(t, billingFields["phone"], "billing.contact must keep field 'phone'")

	// shipping.contact must retain its two fields.
	shippingFields := objectFieldNames(t, resp, shippingContactType)
	assert.True(t, shippingFields["name"], "shipping.contact must keep field 'name'")
	assert.True(t, shippingFields["email"], "shipping.contact must keep field 'email'")
	assert.Len(t, shippingFields, 2, "shipping.contact must have exactly 2 fields")
}

// TestSchemaCollision_Deterministic guards against the Go map-iteration
// nondeterminism that made the original bug intermittent. Running the schema
// generator many times must always produce byte-identical object and scalar
// type output.
func TestSchemaCollision_Deterministic(t *testing.T) {
	const cfgJSON = `{
	  "indices": {
	    "orders": {"mappings": {"properties": {
	      "address": {"type": "nested", "properties": {
	        "street":  {"type": "text"},
	        "city":    {"type": "keyword"},
	        "country": {"type": "keyword"}
	      }}
	    }}},
	    "returns": {"mappings": {"properties": {
	      "address": {"type": "nested", "properties": {
	        "city":    {"type": "keyword"},
	        "country": {"type": "keyword"}
	      }}
	    }}}
	  },
	  "queries": {}
	}`

	var first string
	for i := 0; i < 50; i++ {
		cfg, st := collisionState(t, cfgJSON)
		resp := connector.ParseConfigurationToSchema(cfg, st)

		// json.Marshal sorts map keys, so identical content → identical bytes.
		objects, err := json.Marshal(resp.ObjectTypes)
		require.NoError(t, err)
		scalars, err := json.Marshal(resp.ScalarTypes)
		require.NoError(t, err)
		cur := string(objects) + "|" + string(scalars)

		if i == 0 {
			first = cur
			continue
		}
		assert.Equalf(t, first, cur, "object/scalar type output must be deterministic (iteration %d differed)", i)
	}
}

// TestSchemaCollision_NativeQueryCollision verifies that native queries
// participate in the same global name resolution as regular indices.
func TestSchemaCollision_NativeQueryCollision(t *testing.T) {
	const cfgJSON = `{
	  "indices": {
	    "orders": {"mappings": {"properties": {
	      "address": {"type": "nested", "properties": {
	        "street":  {"type": "text"},
	        "city":    {"type": "keyword"},
	        "country": {"type": "keyword"}
	      }}
	    }}}
	  },
	  "queries": {
	    "recent_orders": {
	      "index": "orders",
	      "query": {"match_all": {}},
	      "return_type": {
	        "kind": "defination",
	        "mappings": {"properties": {
	          "address": {"type": "nested", "properties": {
	            "city":    {"type": "keyword"},
	            "country": {"type": "keyword"}
	          }}
	        }}
	      }
	    }
	  }
	}`

	cfg, st := collisionState(t, cfgJSON)
	resp := connector.ParseConfigurationToSchema(cfg, st)

	ordersAddrType := nestedTypeName(t, resp, "orders", "address")
	nqAddrType := nestedTypeName(t, resp, "recent_orders", "address")

	// The native query's address (2 fields) must not overwrite the index's
	// address (3 fields).
	assert.NotEqual(t, ordersAddrType, nqAddrType,
		"native query address and index address have different shapes and must be separate types")

	ordersFields := objectFieldNames(t, resp, ordersAddrType)
	assert.True(t, ordersFields["street"], "orders.address must keep field 'street'")
	assert.True(t, ordersFields["city"])
	assert.True(t, ordersFields["country"])

	// Collect all field names to check for no unexpected extras.
	allNames := make([]string, 0)
	for n := range ordersFields {
		allNames = append(allNames, n)
	}
	sort.Strings(allNames)
	assert.Equal(t, []string{"city", "country", "street"}, allNames)
}
