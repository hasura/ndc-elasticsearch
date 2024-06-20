package connector

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hasura/ndc-elasticsearch/types"
)

// ParseConfiguration parses the connector's configuration.
func (c *Connector) ParseConfiguration(ctx context.Context, configurationDir string) (*types.Configuration, error) {
	// Parse the configuration file
	configuration, err := GetConfiguration(configurationDir, configFileName)
	if err != nil {
		return nil, err
	}

	return configuration, nil
}

// GetConfiguration reads the configuration file, parses it into a Configuration struct,
// and initializes the native queries.
func GetConfiguration(configurationDir string, configFileName string) (*types.Configuration, error) {
	// Define the path to the configuration file.
	configFilePath := filepath.Join(configurationDir, configFileName)

	file, err := os.Open(configFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %w", configFilePath, err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)

	var configuration types.Configuration

	if err := decoder.Decode(&configuration); err != nil {
		return nil, fmt.Errorf("failed to decode JSON for file %s: %w", configFileName, err)
	}

	if configuration.Indices == nil {
		return nil, fmt.Errorf("unable to parse configuration from %s, run upgrade command to upgrade the configuration", configFilePath)
	}

	// Initialize the native queries
	configuration.Queries, err = parseNativeQueries(&configuration, configurationDir)
	if err != nil {
		return nil, err
	}

	return &configuration, nil
}

// parseNativeQueries parses the native queries in the configuration file and initializes them.
// It sets the 'file' key in the DSL to the absolute path of the file.
func parseNativeQueries(config *types.Configuration, configDir string) (map[string]types.NativeQuery, error) {
	queries := config.Queries
	parsedQueries := make(map[string]types.NativeQuery, len(queries))

	for name, query := range queries {
		var queryFile string
		if query.DSL.File != nil && *query.DSL.File != "" {
			queryFile = filepath.Join(configDir, *query.DSL.File)
		} else if query.DSL.Internal == nil {
			return nil, fmt.Errorf("invalid 'dsl' definition in %s", name)
		}

		if query.Index == "" {
			return nil, fmt.Errorf("missing 'index' value in %s", name)
		}

		returnTyep := query.ReturnType
		if returnTyep == nil {
			return nil, fmt.Errorf("missing 'return_type' value in %s", name)
		}

		parsedQueries[name] = types.NativeQuery{
			DSL: types.DSL{
				File:     &queryFile,
				Internal: query.DSL.Internal,
			},
			Index:      query.Index,
			Arguments:  query.Arguments,
			ReturnType: returnTyep,
		}
	}

	return parsedQueries, nil
}
