package cli

import (
	"encoding/json"
	"os"

	"github.com/hasura/ndc-elasticsearch/elasticsearch"
)

func updateConfiguration() error {
	client, err := elasticsearch.NewClient()
	if err != nil {
		return err
	}

	indices, err := client.GetIndices()
	if err != nil {
		return err
	}

	result, err := client.GetMappings(indices)
	if err != nil {
		return err
	}

	jsonData, err := json.MarshalIndent(result, "", "    ")
	if err != nil {
		return err
	}

	file, err := os.Create("configuration.json")
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
