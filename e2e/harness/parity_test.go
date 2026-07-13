//go:build e2e

package harness

import "testing"

// TestCompareDDNvsES exercises the DDN<->ES result-set parity assertion directly
// with hand-constructed payloads. The ES payloads use the real _search envelope
// ({"hits":{"hits":[{"_source":{...}}]}}) and the DDN payloads use the real
// GraphQL data envelope ({"<model>": [ ...rows... ]}).
//
// The cases prove three things:
//   - NEGATIVE: the assertion CATCHES genuine divergence (row count, field
//     value, and order for ordered queries).
//   - POSITIVE: it does NOT false-fail on the known, expected representation
//     differences (field-name casing, object-as-array nesting, and row order
//     for unordered queries).
//   - SKIP: aggregation-shaped pairs are skipped (row parity does not apply).
func TestCompareDDNvsES(t *testing.T) {
	tests := []struct {
		name         string
		ddn          string
		es           string
		ordered      bool
		wantSkipped  bool
		wantMismatch bool // true => expect a non-empty mismatch string
	}{
		// ---- NEGATIVE: assertion must catch divergence ----
		{
			name: "neg/row_missing_in_ddn",
			// ES has two rows, DDN only one -> row count differs.
			ddn:          `{"products":[{"sku":"PRD-001"}]}`,
			es:           `{"hits":{"hits":[{"_source":{"sku":"PRD-001"}},{"_source":{"sku":"PRD-002"}}]}}`,
			ordered:      true,
			wantSkipped:  false,
			wantMismatch: true,
		},
		{
			name: "neg/row_missing_in_es",
			// DDN has two rows, ES only one -> row count differs.
			ddn:          `{"products":[{"sku":"PRD-001"},{"sku":"PRD-002"}]}`,
			es:           `{"hits":{"hits":[{"_source":{"sku":"PRD-001"}}]}}`,
			ordered:      true,
			wantSkipped:  false,
			wantMismatch: true,
		},
		{
			name: "neg/field_value_differs",
			// Same row count but the inStock/in_stock value disagrees.
			ddn:          `{"products":[{"sku":"PRD-001","inStock":false}]}`,
			es:           `{"hits":{"hits":[{"_source":{"sku":"PRD-001","in_stock":true}}]}}`,
			ordered:      true,
			wantSkipped:  false,
			wantMismatch: true,
		},
		{
			name: "neg/ordered_same_rows_different_order",
			// Identical rows, different order, ordered query -> position-sensitive fail.
			ddn:          `{"products":[{"sku":"PRD-001"},{"sku":"PRD-002"}]}`,
			es:           `{"hits":{"hits":[{"_source":{"sku":"PRD-002"}},{"_source":{"sku":"PRD-001"}}]}}`,
			ordered:      true,
			wantSkipped:  false,
			wantMismatch: true,
		},

		// ---- POSITIVE: must not false-fail on expected representation diffs ----
		{
			name: "pos/field_name_casing",
			// ES snake_case vs DDN camelCase, same data -> equal.
			ddn:          `{"products":[{"sku":"PRD-001","inStock":true}]}`,
			es:           `{"hits":{"hits":[{"_source":{"sku":"PRD-001","in_stock":true}}]}}`,
			ordered:      true,
			wantSkipped:  false,
			wantMismatch: false,
		},
		{
			name: "pos/object_as_array_nesting",
			// ES object {"dest":"US"} vs DDN single-element object array [{"dest":"US"}] -> equal after unwrap.
			ddn:          `{"kibanaSampleDataLogs":[{"geo":[{"dest":"US"}]}]}`,
			es:           `{"hits":{"hits":[{"_source":{"geo":{"dest":"US"}}}]}}`,
			ordered:      true,
			wantSkipped:  false,
			wantMismatch: false,
		},
		{
			name: "pos/unordered_same_rows_different_order",
			// Same rows, different order, unordered query -> order-insensitive equal.
			ddn:          `{"products":[{"sku":"PRD-001"},{"sku":"PRD-002"}]}`,
			es:           `{"hits":{"hits":[{"_source":{"sku":"PRD-002"}},{"_source":{"sku":"PRD-001"}}]}}`,
			ordered:      false,
			wantSkipped:  false,
			wantMismatch: false,
		},
		{
			name: "pos/casing_and_nesting_combined",
			// Both representation diffs at once, same data -> equal.
			ddn:          `{"products":[{"sku":"PRD-001","shipInfo":[{"toCountry":"US"}]}]}`,
			es:           `{"hits":{"hits":[{"_source":{"sku":"PRD-001","ship_info":{"to_country":"US"}}}]}}`,
			ordered:      true,
			wantSkipped:  false,
			wantMismatch: false,
		},

		// ---- SKIP: aggregation-shaped pair ----
		{
			name: "skip/aggregation",
			// ES aggregations object + DDN <model>Aggregate object (no array root) -> skipped.
			ddn:          `{"kibanaSampleDataLogsAggregate":{"bytes":{"avg":5664.75}}}`,
			es:           `{"aggregations":{"avg_bytes":{"value":5664.75}},"hits":{"hits":[]}}`,
			ordered:      false,
			wantSkipped:  true,
			wantMismatch: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			skipped, mismatch := compareDDNvsES([]byte(tt.ddn), []byte(tt.es), tt.ordered)
			if skipped != tt.wantSkipped {
				t.Fatalf("skipped = %v, want %v (mismatch=%q)", skipped, tt.wantSkipped, mismatch)
			}
			if got := mismatch != ""; got != tt.wantMismatch {
				t.Fatalf("mismatch present = %v, want %v (mismatch=%q)", got, tt.wantMismatch, mismatch)
			}
			if tt.wantMismatch {
				t.Logf("correctly caught divergence: %s", mismatch)
			}
		})
	}
}
