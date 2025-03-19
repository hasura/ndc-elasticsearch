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
)

var (
	errCredentialProviderKeyNotSet        = fmt.Errorf("%s is not set", credentailsProviderKeyEnvVar)
	errCredentialProviderMechanismNotSet  = fmt.Errorf("%s is not set", credentailsProviderMechanismEnvVar)
	errCredentialProviderMechanismInvalid = fmt.Errorf("invalid value for %s, should be either \"api-key\" or \"service-token\"", credentailsProviderMechanismEnvVar)
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

func shouldUseCredentialsProvider() bool {
	return os.Getenv(credentialsProviderUri) != ""
}

func getConfigFromCredentialsProvider(ctx context.Context, forceRefresh bool) (*elasticsearch.Config, error) {
	key := os.Getenv(credentailsProviderKeyEnvVar)
	mechanism := os.Getenv(credentailsProviderMechanismEnvVar)
	return useCredentialsProvider(ctx, key, mechanism, forceRefresh)
}

func useCredentialsProvider(ctx context.Context, key string, mechanism string, forceRefresh bool) (*elasticsearch.Config, error) {
	if key == "" {
		return nil, errCredentialProviderKeyNotSet
	}
	if mechanism == "" {
		return nil, errCredentialProviderMechanismNotSet
	}
	if mechanism != "api-key" && mechanism != "service-token" {
		return nil, errCredentialProviderMechanismInvalid
	}

	credential, err := credentials.AcquireCredentials(ctx, key, forceRefresh)
	if err != nil {
		return nil, err
	}

	esConfig := elasticsearch.Config{}
	if mechanism == "api-key" {
		esConfig.APIKey = credential
	} else {
		esConfig.ServiceToken = credential
	}
	return &esConfig, nil
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
