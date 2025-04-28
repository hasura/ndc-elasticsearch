package connector

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/hasura/ndc-sdk-go/schema"
	"github.com/stretchr/testify/assert"
)

const filterTestsPath = "./testdata/filter_tests/"

func TestGetPredicate(t *testing.T) {
	tests := []struct {
		name               string
		gotExpression      string
		expectedColumnPath string
		wantPredicate      string
	}{
		{
			name:               "nested_001",
			expectedColumnPath: "route.departure_airport.location.state",
		},
		{
			name:               "nested_and_002",
			expectedColumnPath: "",
		},
		{
			name:               "003",
			expectedColumnPath: "route.arrival_airport.location.coordinates.elevation",
		},
		{
			name:               "004",
			expectedColumnPath: "route.arrival_airport.terminals",
		},
		{
			name:               "aggregations_005",
			expectedColumnPath: "metric_value",
		},
		{
			name:               "006",
			expectedColumnPath: "name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// setup test data
			gotB, err := os.ReadFile(filepath.Join(filterTestsPath, "get_predicate", tt.name, "got.json"))
			assert.NoError(t, err, "Error reading got.json file")
			tt.gotExpression = string(gotB)

			wantB, err := os.ReadFile(filepath.Join(filterTestsPath, "get_predicate", tt.name, "want.json"))
			assert.NoError(t, err, "Error reading want.json file")
			tt.wantPredicate = string(wantB)

			// Convert tt.expression from JSON string to schema.Expression
			var expression schema.Expression
			err = json.Unmarshal([]byte(tt.gotExpression), &expression)
			assert.NoError(t, err, "Error unmarshalling expression JSON")

			// Convert tt.expectedPredicate from JSON string to schema.Expression
			var expectedPredicate schema.Expression
			err = json.Unmarshal([]byte(tt.wantPredicate), &expectedPredicate)
			assert.NoError(t, err, "Error unmarshalling expectedPredicate JSON")

			// Call getPredicate and validate results
			path, result := getPredicate(expression)
			assert.Equal(t, tt.expectedColumnPath, path)
			assert.Equal(t, expectedPredicate, result)

			// uncomment to update want file
			// err = os.WriteFile(filepath.Join(filterTestsPath, "get_predicate", tt.name, "want.json"), []byte(tt.wantPredicate), 0644)
			// assert.NoError(t, err, "Error writing want file")
		})
	}
}

func TestRequiresNestedFiltering(t *testing.T) {
	tests := []struct {
		name                string
		predicate           schema.Expression
		expectedNested      bool
		expectedNestedField string
	}{
		{
			name: "Valid nested collection",
			predicate: schema.Expression{
				"in_collection": schema.ExistsInCollection{
					"column_name": "my_nested_field",
					"type":        "nested_collection",
				},
			},
			expectedNested:      true,
			expectedNestedField: "my_nested_field",
		},
		{
			name: "Missing in_collection key",
			predicate: schema.Expression{
				"some_other_key": "some_value",
			},
			expectedNested:      false,
			expectedNestedField: "",
		},
		{
			name: "Invalid in_collection type",
			predicate: schema.Expression{
				"in_collection": "invalid_type",
			},
			expectedNested:      false,
			expectedNestedField: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nested, fieldName := requiresNestedFiltering(tt.predicate)
			assert.Equal(t, tt.expectedNested, nested)
			assert.Equal(t, tt.expectedNestedField, fieldName)
		})
	}
}
