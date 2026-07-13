//go:build e2e

package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"
)

// ndcSchema is a minimal projection of the connector's GET /schema response
// (ndc-sdk-go schema.SchemaResponse) — just enough for L3 conformance.
type ndcSchema struct {
	ScalarTypes map[string]struct {
		Representation struct {
			Type string `json:"type"`
		} `json:"representation"`
	} `json:"scalar_types"`
	ObjectTypes map[string]struct {
		Fields map[string]struct {
			Type json.RawMessage `json:"type"`
		} `json:"fields"`
	} `json:"object_types"`
	Collections []struct {
		Name string `json:"name"`
		Type string `json:"type"`
	} `json:"collections"`
}

// FetchConnectorSchema GETs :<port>/schema from the running connector.
func FetchConnectorSchema(ctx context.Context, connectorPort int) (*ndcSchema, error) {
	url := fmt.Sprintf("http://localhost:%d/schema", connectorPort)
	client := &http.Client{Timeout: 30 * time.Second}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s => %d", url, resp.StatusCode)
	}
	var out ndcSchema
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// AssertSchemaConformance performs the L3 assertions:
//
//  1. For each index in configuration.json, GET /<index>/_mapping from ES and
//     deep-equal it against configuration.json.indices.<index>.mappings.
//  2. For each such index, assert the connector /schema has a collection, and
//     that every leaf ES field maps to an NDC field whose scalar type carries
//     the expected representation per the fixed ES->NDC table (typemap.go), and
//     that object/nested containers become object types / arrays of them.
//
// Returns a human-readable list of problems (empty => conformant).
func AssertSchemaConformance(ctx context.Context, s *Stack, es *ESClient) ([]string, error) {
	t := s.startTimer("assert:schema")
	defer t.done()

	cfg, err := loadConfiguration(filepath.Join(s.Workdir, "connector", "configuration.json"))
	if err != nil {
		return nil, err
	}
	sch, err := FetchConnectorSchema(ctx, s.Ports["connector"])
	if err != nil {
		return nil, err
	}

	collByName := map[string]string{} // collection name -> object type
	for _, c := range sch.Collections {
		collByName[c.Name] = c.Type
	}

	var problems []string
	indexNames := make([]string, 0, len(cfg.Indices))
	for k := range cfg.Indices {
		indexNames = append(indexNames, k)
	}
	sort.Strings(indexNames)

	for _, index := range indexNames {
		cfgIndex, _ := cfg.Indices[index].(map[string]interface{})
		cfgMappings, _ := cfgIndex["mappings"].(map[string]interface{})
		if cfgMappings == nil {
			problems = append(problems, fmt.Sprintf("index %q: configuration.json has no mappings", index))
			continue
		}

		// (1) ES _mapping deep-equals configuration mappings.
		if probs := assertMappingEquality(ctx, es, index, cfgMappings); len(probs) > 0 {
			problems = append(problems, probs...)
		}

		// (2) connector schema conformance.
		objType, ok := collByName[index]
		if !ok {
			problems = append(problems, fmt.Sprintf("index %q: no collection in connector /schema", index))
			continue
		}
		props, _ := cfgMappings["properties"].(map[string]interface{})
		problems = append(problems, assertFieldsConform(sch, index, objType, "", props)...)
	}
	return problems, nil
}

// assertMappingEquality compares ES GET /<index>/_mapping against the config
// mappings. ES returns { "<concrete-index>": { "mappings": {...} } } possibly
// for multiple backing indices (data streams); each must equal config mappings.
func assertMappingEquality(ctx context.Context, es *ESClient, index string, cfgMappings map[string]interface{}) []string {
	raw, err := es.GetMapping(ctx, index)
	if err != nil {
		return []string{fmt.Sprintf("index %q: GET _mapping failed: %v", index, err)}
	}
	var problems []string
	for concrete, v := range raw {
		entry, _ := v.(map[string]interface{})
		esMappings, _ := entry["mappings"].(map[string]interface{})
		if !jsonDeepEqual(esMappings, cfgMappings) {
			problems = append(problems, fmt.Sprintf(
				"index %q (backing %q): ES _mapping != configuration.json mappings\n  es:  %s\n  cfg: %s",
				index, concrete, compactJSON(esMappings), compactJSON(cfgMappings)))
		}
	}
	return problems
}

// assertFieldsConform recursively walks the ES mapping properties, checking each
// field against the connector's NDC object type.
func assertFieldsConform(sch *ndcSchema, index, ndcObjType, prefix string, props map[string]interface{}) []string {
	var problems []string
	ot, ok := sch.ObjectTypes[ndcObjType]
	if !ok {
		return []string{fmt.Sprintf("%s: NDC object type %q missing from /schema", index, ndcObjType)}
	}

	names := make([]string, 0, len(props))
	for k := range props {
		names = append(names, k)
	}
	sort.Strings(names)

	for _, fieldName := range names {
		fieldPath := joinPath(prefix, fieldName)
		fieldDef, _ := props[fieldName].(map[string]interface{})
		if fieldDef == nil {
			continue
		}

		ndcField, present := ot.Fields[fieldName]
		if !present {
			problems = append(problems, fmt.Sprintf("%s.%s: leaf/object present in ES mapping but missing from NDC object type %q",
				index, fieldPath, ndcObjType))
			continue
		}
		baseName, isArray, err := resolveNamedType(ndcField.Type)
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s.%s: cannot resolve NDC type: %v", index, fieldPath, err))
			continue
		}

		esType, _ := fieldDef["type"].(string)
		childProps, hasProps := fieldDef["properties"].(map[string]interface{})

		switch {
		case hasProps && (esType == "nested" || esType == "object" || esType == ""):
			// The connector represents EVERY ES object container — `nested`,
			// explicit `object`, and implicit-object (properties with no `type`)
			// — as an ARRAY of a named object type. An ES object field can hold a
			// single object or an array of objects interchangeably, so the
			// connector models them uniformly as arrays (see
			// connector/schema.go: getScalarTypesAndObjects sets obj:true for any
			// field with `properties`, and getNdcObjectFields wraps obj fields in
			// an array type). We assert that behaviour consistently here.
			if !isArray {
				problems = append(problems, fmt.Sprintf("%s.%s: object/nested container should be an ARRAY of object type in NDC, got named %q",
					index, fieldPath, baseName))
			}
			problems = append(problems, assertFieldsConform(sch, index, baseName, fieldPath, childProps)...)

		case esType == "flattened":
			// opaque; connector treats as JSON-ish. No structural assertion.

		case esType != "":
			// leaf scalar
			problems = append(problems, assertScalarField(sch, index, fieldPath, esType, baseName)...)

		default:
			// unknown shape; be lenient but note it.
			problems = append(problems, fmt.Sprintf("%s.%s: unrecognized ES field definition (no type, no properties)", index, fieldPath))
		}
	}
	return problems
}

// assertScalarField checks a leaf field's NDC scalar type name + representation.
func assertScalarField(sch *ndcSchema, index, fieldPath, esType, ndcScalarName string) []string {
	var problems []string

	// The NDC scalar type name is either exactly the ES base type or a compound
	// "baseType.subtype1.subtype2..." for multi-fields (typemap.go). Either way
	// it must be prefixed by the base ES type.
	if ndcScalarName != esType && !strings.HasPrefix(ndcScalarName, esType+".") {
		problems = append(problems, fmt.Sprintf("%s.%s: ES type %q mapped to NDC scalar %q (expected %q or %q.<subtypes>)",
			index, fieldPath, esType, ndcScalarName, esType, esType))
	}

	scalar, ok := sch.ScalarTypes[ndcScalarName]
	if !ok {
		problems = append(problems, fmt.Sprintf("%s.%s: NDC scalar type %q not declared in /schema", index, fieldPath, ndcScalarName))
		return problems
	}
	if wantRepr, known := expectedReprFor(esType); known {
		if scalar.Representation.Type != wantRepr {
			problems = append(problems, fmt.Sprintf("%s.%s: ES type %q => NDC repr %q, expected %q",
				index, fieldPath, esType, scalar.Representation.Type, wantRepr))
		}
	}
	return problems
}

// resolveNamedType unwraps an NDC type (nullable/array/named) to its base named
// type and whether an array was encountered.
func resolveNamedType(raw json.RawMessage) (name string, isArray bool, err error) {
	var t struct {
		Type           string          `json:"type"`
		Name           string          `json:"name"`
		ElementType    json.RawMessage `json:"element_type"`
		UnderlyingType json.RawMessage `json:"underlying_type"`
	}
	cur := raw
	for i := 0; i < 8; i++ {
		if err := json.Unmarshal(cur, &t); err != nil {
			return "", false, err
		}
		switch t.Type {
		case "named":
			return t.Name, isArray, nil
		case "array":
			isArray = true
			cur = t.ElementType
		case "nullable":
			cur = t.UnderlyingType
		default:
			return "", false, fmt.Errorf("unknown NDC type kind %q", t.Type)
		}
		t = struct {
			Type           string          `json:"type"`
			Name           string          `json:"name"`
			ElementType    json.RawMessage `json:"element_type"`
			UnderlyingType json.RawMessage `json:"underlying_type"`
		}{}
	}
	return "", false, fmt.Errorf("NDC type nesting too deep")
}

// ---- configuration.json ----

type configuration struct {
	Indices map[string]interface{} `json:"indices"`
	Queries map[string]interface{} `json:"queries"`
}

func loadConfiguration(path string) (*configuration, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	var c configuration
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

// ---- small helpers ----

func joinPath(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return prefix + "." + name
}

func jsonDeepEqual(a, b interface{}) bool {
	// Normalize through JSON round-trip so numeric/interface types line up.
	an, err1 := roundTrip(a)
	bn, err2 := roundTrip(b)
	if err1 != nil || err2 != nil {
		return reflect.DeepEqual(a, b)
	}
	return reflect.DeepEqual(an, bn)
}

func roundTrip(v interface{}) (interface{}, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var out interface{}
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func compactJSON(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return tail(string(b), 1200)
}
