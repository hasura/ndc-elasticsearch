package connector_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/hasura/ndc-elasticsearch/connector"
	"github.com/hasura/ndc-elasticsearch/types"
	"github.com/stretchr/testify/assert"
)

const testsPath = "../testdata/unit_tests/fields_tests/"

type test struct {
	name          string
	configuration *types.Configuration
	wantSchema    []byte
	state         *types.State
}

var tests = []test{
	{
		name: "identification",
	},
	{
		name: "books",
	},
}

func TestSchema(t *testing.T) {
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			initTest(t, &tt)

			assert.NotNil(t, tt.state, "State is nil")
			assert.NotNil(t, tt.configuration, "Configuration is nil")

			schema := connector.ParseConfigurationToSchema(tt.configuration, tt.state)

			jsonData, err := json.MarshalIndent(schema, "", "  ")
			assert.NoError(t, err, "Error marshalling schema")

			assert.JSONEq(t, string(tt.wantSchema), string(jsonData), "Schema does not match")

			// uncomment to update want file
			// err = os.WriteFile(filepath.Join(testsPath, tt.name, "want_schema.json"), jsonData, 0644)
			// assert.NoError(t, err, "Error writing want_schema file")
		})
	}
}

func initTest(t *testing.T, testCase *test) {
	testCase.state = &types.State{
		TelemetryState:           nil,
		Client:                   nil,
		SupportedSortFields:      make(map[string]interface{}),
		SupportedAggregateFields: make(map[string]interface{}),
		SupportedFilterFields:    make(map[string]interface{}),
		NestedFields:             make(map[string]interface{}),
		ElasticsearchInfo:        nil,
	}

	configurationB, err := os.ReadFile(filepath.Join(testsPath, testCase.name, "configuration.json"))
	assert.NoError(t, err, "Error reading configuration file")

	err = json.Unmarshal(configurationB, &testCase.configuration)
	assert.NoError(t, err, "Error unmarshalling configuration")

	testCase.wantSchema, err = os.ReadFile(filepath.Join(testsPath, testCase.name, "want_schema.json"))
	assert.NoError(t, err, "Error reading want_schema file")
}
