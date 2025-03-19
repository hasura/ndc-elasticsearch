package cli

import (
	"context"
	"encoding/json"
	"path/filepath"

	"github.com/hasura/ndc-elasticsearch/connector"
	"github.com/hasura/ndc-elasticsearch/elasticsearch"
	"github.com/hasura/ndc-elasticsearch/internal"
	"github.com/hasura/ndc-elasticsearch/types"
)

// updateConfiguration updates the configuration file with Elasticsearch mappings.
// It creates a new client, retrieves the indices and mappings from Elasticsearch,
// marshals the mappings into a JSON configuration file, and writes the updated
// configuration to disk atomically.
func updateConfig(ctx context.Context, configDir string) error {
	// Create a new Elasticsearch client.
	client, err := elasticsearch.NewClient(ctx)
	if err != nil {
		return err
	}

	// Get the indices from Elasticsearch.
	indices, err := client.GetIndices(ctx)
	if err != nil {
		return err
	}

	// Get the mappings for the indices from Elasticsearch.
	mappings, err := client.GetMappings(ctx, indices)
	if err != nil {
		return err
	}

	// Get aliases for indices and add them to mappings.
	aliasToIndexMap, err := client.GetAliases(ctx)
	if err != nil {
		return err
	}

	// Add aliases to mappings.
	client.AddAliasesToMappings(ctx, aliasToIndexMap, mappings)

	configPath := filepath.Join(configDir, ConfigFileName)

	// Marshal the mappings into a JSON configuration file.
	configData, err := marshalMappings(configPath, mappings)
	if err != nil {
		return err
	}

	// Write the updated configuration to disk atomically.
	err = internal.WriteJsonFile(configPath, configData)
	if err != nil {
		return err
	}

	return nil
}

// marshalMappings marshals the Elasticsearch mappings into a JSON configuration file.
// It reads the existing configuration file if it exists, or creates a new one with an empty "queries" field.
// It then overwrites the "indices" field with the provided mappings.
// Returns the JSON data as a byte array and any error encountered.
func marshalMappings(configPath string, mappings map[string]interface{}) ([]byte, error) {
	// Define the initial configuration template.
	configuration := &types.Configuration{
		Indices: make(map[string]interface{}),
		Queries: make(map[string]types.NativeQuery),
	}

	// If the configuration file exists, read it using the decoder.
	if internal.FileExists(configPath) {
		var err error
		configuration, err = connector.GetConfiguration(configPath, "")
		if err != nil {
			return nil, err
		}
	}
	// Overwrite the "indices" field with the provided mappings.
	configuration.Indices = mappings

	// Marshal the configuration data into a JSON byte array with indentation.
	return json.MarshalIndent(configuration, "", "  ")
}
