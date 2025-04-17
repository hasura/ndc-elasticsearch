package elasticsearch

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/hasura/ndc-sdk-go/credentials"
)

const esMaxResultSize = 10000
const DEFAULT_RESULT_SIZE_KEY = "esDefaultResultSize"

var (
	credentailsProviderKeyEnvVar       = "ELASTICSEARCH_CREDENTIALS_PROVIDER_KEY"
	credentailsProviderMechanismEnvVar = "ELASTICSEARCH_CREDENTIALS_PROVIDER_MECHANISM"
	credentialsProviderUri             = "HASURA_CREDENTIALS_PROVIDER_URI"
	elasticsearchUrl                   = "ELASTICSEARCH_URL"
)

var (
	errCredentialProviderKeyNotSet        = fmt.Errorf("%s is not set", credentailsProviderKeyEnvVar)
	errCredentialProviderMechanismNotSet  = fmt.Errorf("%s is not set", credentailsProviderMechanismEnvVar)
	errCredentialProviderMechanismInvalid = fmt.Errorf("invalid value for %s, should be either \"api-key\" or \"service-token\"", credentailsProviderMechanismEnvVar)
	errElasticsearchUrlNotSet             = fmt.Errorf("%s is not set", elasticsearchUrl)
)

// getConfigFromEnv retrieves elastic search configuration from environment variables.
func getConfigFromEnv() (*elasticsearch.Config, error) {
	esConfig, err := getBaseConfig()
	if err != nil {
		return nil, err
	}

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

	return esConfig, nil
}

func shouldUseCredentialsProvider() bool {
	return os.Getenv(credentialsProviderUri) != ""
}

func getConfigFromCredentialsProvider(ctx context.Context, forceRefresh bool) (*elasticsearch.Config, error) {
	esConfig, err := getBaseConfig()
	if err != nil {
		return nil, err
	}

	key := os.Getenv(credentailsProviderKeyEnvVar)
	mechanism := os.Getenv(credentailsProviderMechanismEnvVar)
	err = setupCredentailsUsingCredentialsProvider(ctx, esConfig, key, mechanism, forceRefresh)
	if err != nil {
		return nil, err
	}
	return esConfig, nil
}

// setupCredentailsUsingCredentialsProvider sets up the credentials in the elasticsearch config.
// It returns the updated config.
func setupCredentailsUsingCredentialsProvider(ctx context.Context, esConfig *elasticsearch.Config, key string, mechanism string, forceRefresh bool) error {
	if key == "" {
		return errCredentialProviderKeyNotSet
	}
	if mechanism == "" {
		return errCredentialProviderMechanismNotSet
	}
	if mechanism != "api-key" && mechanism != "service-token" {
		return errCredentialProviderMechanismInvalid
	}

	credential, err := credentials.AcquireCredentials(ctx, key, forceRefresh)
	if err != nil {
		return err
	}

	if mechanism == "api-key" {
		esConfig.APIKey = credential
	} else {
		esConfig.ServiceToken = credential
	}
	return nil
}

func GetDefaultResultSize() int {
	defaultResultSize := os.Getenv("ELASTICSEARCH_DEFAULT_RESULT_SIZE")
	if defaultResultSize == "" {
		return esMaxResultSize
	}

	size, err := strconv.Atoi(defaultResultSize)
	if err != nil {
		return esMaxResultSize
	}

	return size
}

// getBaseConfig returns a new elasticsearch client with only the address set.
// This function should be used to setup the config with properties
// that will be common across all configs (credentials provieder based configs or env based configs).
func getBaseConfig() (*elasticsearch.Config, error) {
	esConfig := elasticsearch.Config{}

	// Read the address
	address := os.Getenv(elasticsearchUrl)
	if address == "" {
		return nil, errElasticsearchUrlNotSet
	}

	// Split the address by comma
	addresses := make([]string, 0)
	addresses = append(addresses, strings.Split(address, ",")...)
	esConfig.Addresses = addresses

	return &esConfig, nil
}
