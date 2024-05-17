package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func validate(configPath string) error {
	configFilePath := filepath.Join(configPath, ConfigFileName)
	file, err := os.Open(configFilePath)
	if err != nil {
		return fmt.Errorf("error opening file: %v", err)
	}
	defer file.Close()

	// Decode the JSON file
	var data map[string]interface{}
	err = json.NewDecoder(file).Decode(&data)
	if err != nil {
		return fmt.Errorf("error decoding JSON: %v", err)
	}

	// Validate the mappings
	for indexName, indexData := range data {
		// Assuming each index in the file has a "mappings" key
		mappings, ok := indexData.(map[string]interface{})["mappings"]
		if !ok {
			return fmt.Errorf("index %s is missing 'mappings' key", indexName)
		}
		_, ok = mappings.(map[string]interface{})["properties"]
		if !ok {
			return fmt.Errorf("index %s is missing 'properties' key", indexName)
		}

	}
	return nil
}
