package cli

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/hasura/ndc-elasticsearch/internal"
	"github.com/hasura/ndc-elasticsearch/types"
)

const ConfigFileName = "configuration.json"

// initializeConfig creates a configuration file at the specified path.
// It returns an error if the configuration file already exists.
// It creates an empty configuration file with "indices" and "queries" fields.
func initializeConfig(path string) error {
	configPath := filepath.Join(path, ConfigFileName)

	// Return an error if the configuration file already exists
	if internal.FileExists(configPath) {
		return fmt.Errorf("configuration file already exists at %s", configPath)
	}

	// Create an empty configuration file
	config := types.Configuration{
		Indices: make(map[string]interface{}),
		Queries: make(map[string]types.NativeQuery),
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	// Write the configuration file
	err = internal.WriteJsonFile(configPath, data)
	return err
}
