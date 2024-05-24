package elasticsearch

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/elastic/go-elasticsearch/v8"
)

// getConfigFromEnv retrieves elastic search configuration from environment variables.
func getConfigFromEnv() (*elasticsearch.Config, error) {
	esConfig := elasticsearch.Config{}

	// Read the address
	address := os.Getenv("ELASTICSEARCH_URL")
	if address == "" {
		return nil, errors.New("ELASTICSEARCH_URL is not set")
	}

	// Split the address by comma
	addresses := make([]string, 0)
	addresses = append(addresses, strings.Split(address, ",")...)
	esConfig.Addresses = addresses

	// Read the credentials if provided
	username := os.Getenv("ELASTICSEARCH_USERNAME")
	password := os.Getenv("ELASTICSEARCH_PASSWORD")
	apiKey := os.Getenv("ELASTICSEARCH_API_KEY")

	if apiKey == "" && (username == "" || password == "") {
		return nil, errors.New("either username and password or apiKey should be provided")
	}
	esConfig.APIKey = apiKey
	esConfig.Username = username
	esConfig.Password = password

	// Read the CA certificate if provided
	caCertPath := os.Getenv("ELASTICSEARCH_CA_CERT_PATH")
	if caCertPath != "" {
		cert, err := os.ReadFile(caCertPath)
		if err != nil {
			return nil, fmt.Errorf("error reading CA certificate. Path: %s, Error: %v", caCertPath, err)
		}

		esConfig.CACert = cert
	}

	return &esConfig, nil
}
