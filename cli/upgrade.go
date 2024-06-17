package cli

import (
	"encoding/json"
	"path/filepath"

	"github.com/hasura/ndc-elasticsearch/internal"
	"github.com/hasura/ndc-elasticsearch/types"
)

// Upgrade upgrades the configuration directory to be compatible with the latest connector version.
func upgradeConfig(sourceDir string, destDir string) (bool, error) {
	sourceDir = filepath.Join(sourceDir, ConfigFileName)
	destDir = filepath.Join(destDir, ConfigFileName)
	oldConfig, err := internal.ReadJsonFileUsingDecoder(sourceDir)
	if err != nil {
		return false, err
	}
	if _, ok := oldConfig["indices"].(map[string]interface{}); ok {
		return false, nil
	}
	upgradedConfig := upgradeToLatest(oldConfig)

	jsonData, err := json.MarshalIndent(upgradedConfig, "", "  ")
	if err != nil {
		return false, nil
	}

	return true, internal.WriteJsonFile(destDir, jsonData)
}

// upgradeToLatest upgrades a configuration file from an older version of the connector to the latest version.
func upgradeToLatest(oldConfig map[string]interface{}) types.Configuration {
	return types.Configuration{
		Indices: oldConfig,
		Queries: map[string]types.NativeQuery{},
	}
}
