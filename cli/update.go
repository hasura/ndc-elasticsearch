package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/hasura/ndc-elasticsearch/elasticsearch"
)

// updateConfiguration updates the configuration file with the mappings retrieved from Elasticsearch.
func updateConfiguration(ctx context.Context, configPath string) error {
	client, err := elasticsearch.NewClient()
	if err != nil {
		return err
	}

	indices, err := client.GetIndices(ctx)
	if err != nil {
		return err
	}

	result, err := client.GetMappings(ctx, indices)
	if err != nil {
		return err
	}

	jsonData, err := json.MarshalIndent(result, "", "    ")
	if err != nil {
		return err
	}

	configFilePath := filepath.Join(configPath, ConfigFileName)
	file, err := os.Create(configFilePath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.Write(jsonData)
	if err != nil {
		return err
	}

	return nil
}
