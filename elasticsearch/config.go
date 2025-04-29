package elasticsearch

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"net/http"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/hasura/ndc-sdk-go/credentials"
)

const esMaxResultSize = 10000
const DEFAULT_RESULT_SIZE_KEY = "esDefaultResultSize"

var (
	credentialsProviderKeyEnvVar       = "ELASTICSEARCH_CREDENTIALS_PROVIDER_KEY"
	credentialsProviderMechanismEnvVar = "ELASTICSEARCH_CREDENTIALS_PROVIDER_MECHANISM"
	credentialsProviderUriEnvVar       = "HASURA_CREDENTIALS_PROVIDER_URI"
	elasticsearchUrlEnvVar             = "ELASTICSEARCH_URL"

	// Credentials provider mechanisms
	apiKeyCredentialsProviderMechanism       = "api-key"
	serviceTokenCredentialsProviderMechanism = "service-token"
	bearerTokenCredentialsProviderMechanism  = "bearer-token"
)

var (
	errCredentialProviderKeyNotSet        = fmt.Errorf("%s is not set", credentialsProviderKeyEnvVar)
	errCredentialProviderMechanismNotSet  = fmt.Errorf("%s is not set", credentialsProviderMechanismEnvVar)
	errCredentialProviderMechanismInvalid = fmt.Errorf("invalid value for %s, should be either \"%s\" or \"%s\" or \"%s\"", credentialsProviderMechanismEnvVar, apiKeyCredentialsProviderMechanism, serviceTokenCredentialsProviderMechanism, bearerTokenCredentialsProviderMechanism)
	errElasticsearchUrlNotSet             = fmt.Errorf("%s is not set", elasticsearchUrlEnvVar)
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
	return os.Getenv(credentialsProviderUriEnvVar) != ""
}

func getConfigFromCredentialsProvider(ctx context.Context, forceRefresh bool) (*elasticsearch.Config, error) {
	esConfig, err := getBaseConfig()
	if err != nil {
		return nil, err
	}

	key := os.Getenv(credentialsProviderKeyEnvVar)
	mechanism := os.Getenv(credentialsProviderMechanismEnvVar)
	err = setupCredentialsUsingCredentialsProvider(ctx, esConfig, key, mechanism, forceRefresh)
	if err != nil {
		return nil, err
	}
	return esConfig, nil
}

// setupCredentialsUsingCredentialsProvider sets up the credentials in the elasticsearch config.
// It returns the updated config.
func setupCredentialsUsingCredentialsProvider(ctx context.Context, esConfig *elasticsearch.Config, key string, mechanism string, forceRefresh bool) error {
	if key == "" {
		return errCredentialProviderKeyNotSet
	}
	if mechanism == "" {
		return errCredentialProviderMechanismNotSet
	}
	if mechanism != apiKeyCredentialsProviderMechanism && mechanism != serviceTokenCredentialsProviderMechanism && mechanism != bearerTokenCredentialsProviderMechanism {
		return errCredentialProviderMechanismInvalid
	}

	credential, err := credentials.AcquireCredentials(ctx, key, forceRefresh)
	if err != nil {
		return err
	}

	if mechanism == apiKeyCredentialsProviderMechanism {
		esConfig.APIKey = credential
	} else if mechanism == serviceTokenCredentialsProviderMechanism {
		esConfig.ServiceToken = credential
	} else {
		esConfig.Header.Add("Authorization", fmt.Sprintf("Bearer %s", credential))
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
	esConfig := elasticsearch.Config{
		Header: http.Header{},
	}

	// Read the address
	address := os.Getenv(elasticsearchUrlEnvVar)
	if address == "" {
		return nil, errElasticsearchUrlNotSet
	}

	// Split the address by comma
	addresses := make([]string, 0)
	addresses = append(addresses, strings.Split(address, ",")...)
	esConfig.Addresses = addresses

	return &esConfig, nil
}
