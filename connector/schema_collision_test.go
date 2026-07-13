package connector_test

import (
	"encoding/json"
	"testing"

	"github.com/hasura/ndc-elasticsearch/connector"
	"github.com/hasura/ndc-elasticsearch/types"
	"github.com/stretchr/testify/assert"
)

// newCollisionState builds a fresh State for a configuration JSON literal.
func newCollisionState(t *testing.T, configJSON string) *types.State {
	t.Helper()
	var cfg types.Configuration
	assert.NoError(t, json.Unmarshal([]byte(configJSON), &cfg), "unmarshal configuration")
	return &types.State{
		SupportedSortFields:      make(map[string]interface{}),
		SupportedAggregateFields: make(map[string]interface{}),
		SupportedFilterFields:    make(map[string]interface{}),
		NestedFields:             make(map[string]interface{}),
		Configuration:            &cfg,
	}
}

// objectTypeName returns the underlying named type of a collection's field,
// unwrapping array/nullable wrappers from the encoded NDC type.
func objectTypeName(t *testing.T, schemaJSON map[string]interface{}, objectType, field string) string {
	t.Helper()
	objectTypes := schemaJSON["object_types"].(map[string]interface{})
	ot, ok := objectTypes[objectType].(map[string]interface{})
	assert.Truef(t, ok, "object type %q present", objectType)
	fields := ot["fields"].(map[string]interface{})
	f, ok := fields[field].(map[string]interface{})
	assert.Truef(t, ok, "field %q present on %q", field, objectType)
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

func fieldNames(t *testing.T, schemaJSON map[string]interface{}, objectType string) map[string]bool {
	t.Helper()
	objectTypes := schemaJSON["object_types"].(map[string]interface{})
	ot, ok := objectTypes[objectType].(map[string]interface{})
	assert.Truef(t, ok, "object type %q present", objectType)
	out := map[string]bool{}
	for name := range ot["fields"].(map[string]interface{}) {
		out[name] = true
	}
	return out
}

func schemaAsMap(t *testing.T, cfg *types.Configuration, st *types.State) (map[string]interface{}, []byte) {
	t.Helper()
	resp := connector.ParseConfigurationToSchema(cfg, st)
	raw, err := json.Marshal(resp)
	assert.NoError(t, err)
	var m map[string]interface{}
	assert.NoError(t, json.Unmarshal(raw, &m))
	return m, raw
}

// TestObjectTypeNameCollision_DifferentShapes reproduces the customer bug
// (ticket #14975 / ENT-82): two indices each define a nested object named
// `subject`, but one carries extra leaf fields. Previously the two object types
// were both written to ObjectTypes["subject"] and the last writer won, silently
// dropping the extra fields nondeterministically. Each collection must now keep
// all of its fields, and the result must be deterministic.
func TestObjectTypeNameCollision_DifferentShapes(t *testing.T) {
	const cfgJSON = `{
	  "indices": {
	    "indexOne": {"mappings": {"properties": {"subject": {"type": "nested", "properties": {
	      "alternateAccountIdentifier": {"type": "text", "fields": {"keyword": {"type": "keyword", "ignore_above": 256}}},
	      "businessSystemCode": {"type": "text", "fields": {"keyword": {"type": "keyword", "ignore_above": 256}}},
	      "type": {"type": "text", "fields": {"keyword": {"type": "keyword", "ignore_above": 256}}}
	    }}}}},
	    "indexTwo": {"mappings": {"properties": {"subject": {"type": "nested", "properties": {
	      "type": {"type": "text", "fields": {"keyword": {"type": "keyword", "ignore_above": 256}}}
	    }}}}}
	  },
	  "queries": {}
	}`

	st := newCollisionState(t, cfgJSON)
	schemaMap, _ := schemaAsMap(t, st.Configuration, st)

	oneSubject := objectTypeName(t, schemaMap, "indexOne", "subject")
	twoSubject := objectTypeName(t, schemaMap, "indexTwo", "subject")

	// Different structures must resolve to different object types (no clobber).
	assert.NotEqual(t, oneSubject, twoSubject, "colliding subject types must be disambiguated")

	// Names are always fully-qualified as `index.path.to.field`.
	assert.Equal(t, "indexOne.subject", oneSubject)
	assert.Equal(t, "indexTwo.subject", twoSubject)

	oneFields := fieldNames(t, schemaMap, oneSubject)
	assert.True(t, oneFields["alternateAccountIdentifier"], "indexOne.subject must keep alternateAccountIdentifier")
	assert.True(t, oneFields["businessSystemCode"], "indexOne.subject must keep businessSystemCode")
	assert.True(t, oneFields["type"], "indexOne.subject must keep type")

	twoFields := fieldNames(t, schemaMap, twoSubject)
	assert.True(t, twoFields["type"], "indexTwo.subject must keep type")
	assert.Len(t, twoFields, 1, "indexTwo.subject has exactly one field")
}

// TestObjectTypeNameCollision_AuditIndexFalse is the audit-object form of the
// same bug: the `index:false` fields exist only in the fuller definition. They
// must survive regardless of which index "wins".
func TestObjectTypeNameCollision_AuditIndexFalse(t *testing.T) {
	const cfgJSON = `{
	  "indices": {
	    "auditFull": {"mappings": {"properties": {"audit": {"type": "nested", "properties": {
	      "dtLastUpdated": {"type": "date"},
	      "hash": {"type": "text", "index": false},
	      "mode": {"type": "text", "index": false},
	      "source": {"type": "date", "index": false}
	    }}}}},
	    "auditMinimal": {"mappings": {"properties": {"audit": {"type": "nested", "properties": {
	      "dtLastUpdated": {"type": "date"}
	    }}}}}
	  },
	  "queries": {}
	}`

	st := newCollisionState(t, cfgJSON)
	schemaMap, _ := schemaAsMap(t, st.Configuration, st)

	fullAudit := objectTypeName(t, schemaMap, "auditFull", "audit")
	assert.Equal(t, "auditFull.audit", fullAudit, "nested object types are always fully-qualified")
	full := fieldNames(t, schemaMap, fullAudit)
	for _, f := range []string{"dtLastUpdated", "hash", "mode", "source"} {
		assert.Truef(t, full[f], "auditFull.audit must keep index:false field %q", f)
	}
}

// TestObjectTypeNameCollision_IdenticalStillFullyQualified verifies that even
// two structurally-identical `subject` objects (e.g. an index and its alias) are
// each emitted under their own fully-qualified name — never collapsed to a bare
// "subject". Collapsing would be unstable: it would depend on what other indices
// happen to exist, so adding/removing an index could rename the type. Both types
// still carry all their fields.
func TestObjectTypeNameCollision_IdenticalStillFullyQualified(t *testing.T) {
	const cfgJSON = `{
	  "indices": {
	    "indexOne": {"mappings": {"properties": {"subject": {"type": "nested", "properties": {
	      "type": {"type": "text", "fields": {"keyword": {"type": "keyword", "ignore_above": 256}}}
	    }}}}},
	    "indexAlias": {"mappings": {"properties": {"subject": {"type": "nested", "properties": {
	      "type": {"type": "text", "fields": {"keyword": {"type": "keyword", "ignore_above": 256}}}
	    }}}}}
	  },
	  "queries": {}
	}`

	st := newCollisionState(t, cfgJSON)
	schemaMap, _ := schemaAsMap(t, st.Configuration, st)

	assert.Equal(t, "indexOne.subject", objectTypeName(t, schemaMap, "indexOne", "subject"),
		"nested object types are always fully-qualified, even when structurally identical across indices")
	assert.Equal(t, "indexAlias.subject", objectTypeName(t, schemaMap, "indexAlias", "subject"))

	// The bare name must never be emitted as an object type.
	objectTypes := schemaMap["object_types"].(map[string]interface{})
	_, bare := objectTypes["subject"]
	assert.False(t, bare, "bare 'subject' object type must not be emitted")

	// Both fully-qualified types keep their field.
	assert.True(t, fieldNames(t, schemaMap, "indexOne.subject")["type"])
	assert.True(t, fieldNames(t, schemaMap, "indexAlias.subject")["type"])
}

// TestObjectTypeName_FullyQualifiedSingleIndex pins the stability property with
// the simplest case: a single index with a nested `subject` must produce the
// object type "products.subject" (fully-qualified), NOT bare "subject". This is
// what guarantees that later adding another index can never rename this type.
func TestObjectTypeName_FullyQualifiedSingleIndex(t *testing.T) {
	const cfgJSON = `{
	  "indices": {
	    "products": {"mappings": {"properties": {"subject": {"type": "nested", "properties": {
	      "type": {"type": "text"}
	    }}}}}
	  },
	  "queries": {}
	}`

	st := newCollisionState(t, cfgJSON)
	schemaMap, _ := schemaAsMap(t, st.Configuration, st)

	assert.Equal(t, "products.subject", objectTypeName(t, schemaMap, "products", "subject"),
		"a nested object type must be fully-qualified even when its bare name is unique")

	objectTypes := schemaMap["object_types"].(map[string]interface{})
	_, bare := objectTypes["subject"]
	assert.False(t, bare, "bare 'subject' object type must not be emitted")
}

// TestObjectTypeNameCollision_Deterministic runs the generator many times and
// asserts byte-identical output, guarding against the Go map-iteration-order
// nondeterminism that made the original bug intermittent.
func TestObjectTypeNameCollision_Deterministic(t *testing.T) {
	const cfgJSON = `{
	  "indices": {
	    "indexOne": {"mappings": {"properties": {"subject": {"type": "nested", "properties": {
	      "alternateAccountIdentifier": {"type": "text"},
	      "businessSystemCode": {"type": "text"},
	      "type": {"type": "text"}
	    }}}}},
	    "indexTwo": {"mappings": {"properties": {"subject": {"type": "nested", "properties": {
	      "type": {"type": "text"}
	    }}}}}
	  },
	  "queries": {}
	}`

	// Compare the object/scalar type content (what this fix governs). Map JSON
	// marshaling sorts keys, so identical content yields byte-identical output.
	// (The Collections *slice* order is appended during map iteration and is a
	// separate, pre-existing nondeterminism that does not affect field content.)
	var first string
	for i := 0; i < 50; i++ {
		st := newCollisionState(t, cfgJSON)
		resp := connector.ParseConfigurationToSchema(st.Configuration, st)
		objects, err := json.Marshal(resp.ObjectTypes)
		assert.NoError(t, err)
		scalars, err := json.Marshal(resp.ScalarTypes)
		assert.NoError(t, err)
		cur := string(objects) + "|" + string(scalars)
		if i == 0 {
			first = cur
			continue
		}
		assert.Equalf(t, first, cur, "object/scalar type output must be deterministic (run %d differed)", i)
	}
}
