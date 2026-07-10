//go:build e2e

package harness

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// This file implements the DDN<->ES result-set parity check that enforces the
// suite's stated purpose: "GraphQL queries return the same result set as the
// equivalent query sent directly to Elasticsearch."
//
// A naive deep-equal of the raw DDN and ES JSON produces false failures because
// the two sides represent the same data differently in known, expected ways.
// compareDDNvsES normalizes those differences before comparing:
//
//   - Envelope shape: DDN returns rows under the model field
//     ({"<model>": [ ...rows... ]}); ES returns them under hits.hits[]._source.
//     extractDDNRows / extractESRows pull the ordered row list from each side.
//   - Field-name casing: DDN/GraphQL normalizes ES field names to camelCase
//     (ES in_stock -> DDN inStock); canonicalizeRow folds keys on both sides to
//     a canonical snake_case form recursively.
//   - Object nesting: the connector models every ES object container (explicit
//     object, nested, and implicit-object properties) as an ARRAY of a named
//     object type, so a raw ES value {"dest":"US"} appears as [{"dest":"US"}]
//     on the DDN side. canonicalizeRow unwraps single-element object arrays
//     recursively so the two sides line up.
//   - Ordering: rowSetsEqual is position-sensitive when the query pins an
//     ordering (see queryIsOrdered) and an order-insensitive multiset
//     comparison otherwise.
//
// Aggregation queries are shaped completely differently on the two sides
// (DDN {"<model>Aggregate": {...}} vs ES {"aggregations": {...}}), so a
// row-parity check cannot apply; compareDDNvsES detects and skips them.

// compareDDNvsES enforces DDN<->ES result-set parity. It returns skipped=true
// for aggregation-shaped results (which row parity cannot describe), and
// otherwise returns a non-empty mismatch string describing the first divergence
// found (empty when the result sets match). ordered selects position-sensitive
// vs. order-insensitive row comparison.
func compareDDNvsES(ddnData, esBody []byte, ordered bool) (skipped bool, mismatch string) {
	if isAggregationResult(ddnData, esBody) {
		return true, ""
	}
	ddnRows, ok := extractDDNRows(ddnData)
	if !ok {
		return false, "could not extract DDN row list from result envelope"
	}
	esRows, ok := extractESRows(esBody)
	if !ok {
		return false, "could not extract ES row list from hits.hits[]._source"
	}
	if equal, detail := rowSetsEqual(ddnRows, esRows, ordered); !equal {
		return false, "DDN result set diverges from equivalent ES result set: " + detail
	}
	return false, ""
}

// queryIsOrdered reports whether the query pins a result ordering — a GraphQL
// order_by argument or an ES sort clause. When it does, the DDN<->ES comparison
// is position-sensitive; otherwise an order-insensitive comparison is used.
func queryIsOrdered(q Query) bool {
	if strings.Contains(q.GraphQL, "order_by") {
		return true
	}
	var body map[string]json.RawMessage
	if err := json.Unmarshal(q.ESSearch, &body); err == nil {
		if _, ok := body["sort"]; ok {
			return true
		}
	}
	return false
}

// extractDDNRows returns the ordered row list from a DDN GraphQL data payload of
// shape {"<model>": [ ...rows... ]}. ok is false when the payload is not
// row-shaped (e.g. an aggregation {"<model>Aggregate": {...}}). Keys are visited
// in sorted order so selection is deterministic if more than one root field is
// present.
func extractDDNRows(ddnData []byte) ([]interface{}, bool) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(ddnData, &top); err != nil {
		return nil, false
	}
	keys := make([]string, 0, len(top))
	for k := range top {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		var arr []interface{}
		if err := json.Unmarshal(top[k], &arr); err == nil {
			return arr, true
		}
	}
	return nil, false
}

// extractESRows returns the ordered list of _source objects from a raw ES
// _search response (hits.hits[]._source). ok is false when the body is not a
// hits-shaped search response.
func extractESRows(esBody []byte) ([]interface{}, bool) {
	var top struct {
		Hits struct {
			Hits []struct {
				Source interface{} `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.Unmarshal(esBody, &top); err != nil {
		return nil, false
	}
	// Distinguish "no hits envelope at all" from "zero hits": require the key.
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(esBody, &probe); err != nil {
		return nil, false
	}
	if _, ok := probe["hits"]; !ok {
		return nil, false
	}
	rows := make([]interface{}, 0, len(top.Hits.Hits))
	for _, h := range top.Hits.Hits {
		rows = append(rows, h.Source)
	}
	return rows, true
}

// isAggregationResult reports whether the paired results are aggregation-shaped
// (and thus not amenable to a row-parity comparison): a non-empty ES
// "aggregations" object, or a DDN payload with no array-valued root field.
func isAggregationResult(ddnData, esBody []byte) bool {
	var esTop map[string]json.RawMessage
	if json.Unmarshal(esBody, &esTop) == nil {
		if aggs, ok := esTop["aggregations"]; ok {
			var m map[string]interface{}
			if json.Unmarshal(aggs, &m) == nil && len(m) > 0 {
				return true
			}
		}
	}
	if _, ok := extractDDNRows(ddnData); !ok {
		return true
	}
	return false
}

// rowSetsEqual compares two row lists after canonicalizing each row. When
// ordered is true the comparison is position-sensitive; otherwise it is an
// order-insensitive multiset comparison. detail describes the first divergence.
func rowSetsEqual(ddnRows, esRows []interface{}, ordered bool) (equal bool, detail string) {
	if len(ddnRows) != len(esRows) {
		return false, fmt.Sprintf("row count differs: DDN=%d ES=%d", len(ddnRows), len(esRows))
	}
	ddnCanon := make([]string, len(ddnRows))
	esCanon := make([]string, len(esRows))
	for i := range ddnRows {
		ddnCanon[i] = canonicalJSON(ddnRows[i])
	}
	for i := range esRows {
		esCanon[i] = canonicalJSON(esRows[i])
	}
	if !ordered {
		sort.Strings(ddnCanon)
		sort.Strings(esCanon)
	}
	for i := range ddnCanon {
		if ddnCanon[i] != esCanon[i] {
			return false, fmt.Sprintf("row %d differs:\n  DDN: %s\n   ES: %s", i, ddnCanon[i], esCanon[i])
		}
	}
	return true, ""
}

// canonicalJSON canonicalizes a decoded JSON value and marshals it to a string.
// json.Marshal sorts map keys, so the output is a stable canonical form usable
// for equality and sorting.
func canonicalJSON(v interface{}) string {
	b, err := json.Marshal(canonicalizeRow(v))
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(b)
}

// canonicalizeRow recursively normalizes a decoded JSON value so DDN and ES
// representations of the same data compare equal: map keys are folded to
// snake_case, and single-element arrays whose sole element is an object are
// unwrapped to that object (the connector's object-as-array modeling).
func canonicalizeRow(v interface{}) interface{} {
	switch t := v.(type) {
	case map[string]interface{}:
		out := make(map[string]interface{}, len(t))
		for k, val := range t {
			out[toSnakeCase(k)] = canonicalizeRow(val)
		}
		return out
	case []interface{}:
		arr := make([]interface{}, len(t))
		for i, e := range t {
			arr[i] = canonicalizeRow(e)
		}
		if len(arr) == 1 {
			if _, ok := arr[0].(map[string]interface{}); ok {
				return arr[0]
			}
		}
		return arr
	default:
		return v
	}
}

// toSnakeCase folds a camelCase / PascalCase identifier to snake_case. It is
// idempotent for identifiers that are already snake_case or lower-case, so
// applying it to both the DDN (camelCase) and ES (snake_case) sides yields a
// common canonical form.
func toSnakeCase(s string) string {
	var b strings.Builder
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				b.WriteByte('_')
			}
			b.WriteRune(r - 'A' + 'a')
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}
