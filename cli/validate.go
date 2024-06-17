package cli

import (
	"encoding/json"
	"fmt"

	"github.com/hasura/ndc-elasticsearch/connector"
	"github.com/hasura/ndc-elasticsearch/internal"
	"github.com/hasura/ndc-elasticsearch/types"
)

// validate validates the configuration file.
// validate validates the configuration file. It parses the configuration file,
// validates the mappings of the indices, and validates the native queries.
func validateConfig(configPath string) error {
	// Parse the configuration file
	configuration, err := connector.GetConfiguration(configPath, ConfigFileName)
	if err != nil {
		return err
	}

	// Validate the mappings of the indices
	err = validateMappings(configuration.Indices)
	if err != nil {
		return err
	}

	// Validate the native queries
	err = validateNativeQueries(configuration.Queries)
	if err != nil {
		return err
	}

	return nil
}

// validateMappings validates the mappings of the indices in the configuration file.
func validateMappings(mappings map[string]interface{}) error {
	for indexName, indexData := range mappings {
		// Check if the index data has a 'mappings' key
		mappings, ok := indexData.(map[string]interface{})["mappings"]
		if !ok {
			return fmt.Errorf("index %s is missing 'mappings' key", indexName)
		}
		// Check if the 'mappings' value has a 'properties' key
		_, ok = mappings.(map[string]interface{})["properties"]
		if !ok {
			return fmt.Errorf("index %s is missing 'properties' key", indexName)
		}
	}
	return nil
}

// validateNativeQueries validates the native queries in the configuration file.
// It checks for the presence of required keys and their types.
func validateNativeQueries(nativeQueries map[string]types.NativeQuery) error {
	for queryName, queryConfig := range nativeQueries {
		dsl := queryConfig.DSL

		if dsl.File != nil && *dsl.File != "" {
			_, err := internal.ReadJsonFileUsingDecoder(*dsl.File)
			if err != nil {
				return fmt.Errorf("invalid 'file' value in %s: %w", queryName, err)
			}
		} else if dsl.Internal != nil {
			_, err := json.Marshal(*dsl.Internal)
			if err != nil {
				return fmt.Errorf("invalid 'internal' value in %s: %w", queryName, err)
			}
		} else {
			return fmt.Errorf("missing 'file' or 'internal' key for 'dsl' in %s", queryName)
		}

		if queryConfig.Index == "" {
			return fmt.Errorf("missing 'index' value in %s", queryName)
		}

		if queryConfig.ReturnType == nil {
			return fmt.Errorf("missing 'return_type' value in %s", queryName)
		}

		if queryConfig.ReturnType.Kind != "defination" && queryConfig.ReturnType.Kind != "index" {
			return fmt.Errorf("invalid 'kind' value '%s' in %s", queryConfig.ReturnType.Kind, queryName)
		}

		if queryConfig.ReturnType.Kind == "defination" && queryConfig.ReturnType.Mappings == nil {
			return fmt.Errorf("missing 'mappings' value for kind 'defination' in %s", queryName)
		}
	}

	return nil
}
