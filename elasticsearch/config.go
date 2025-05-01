package elasticsearch

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"io"
	"bytes"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/hasura/ndc-sdk-go/credentials"
	estransport "github.com/elastic/elastic-transport-go/v8/elastictransport"
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
	fmt.Printf("Getting Configuration from env vars for username and password\n")
	esConfig, err := getBaseConfig()
	if err != nil {
		fmt.Printf("Error getting base config: %v\n", err)
		return nil, err
	}

	// Read the credentials if provided
	username := os.Getenv("ELASTICSEARCH_USERNAME")
	password := os.Getenv("ELASTICSEARCH_PASSWORD")
	apiKey := os.Getenv("ELASTICSEARCH_API_KEY")

	if apiKey == "" && (username == "" || password == "") {
		fmt.Printf("Error: either username and password or apiKey should be provided\n")
		return nil, errors.New("either username and password or apiKey should be provided")
	}
	esConfig.APIKey = apiKey
	esConfig.Username = username
	esConfig.Password = password

	return esConfig, nil
}

func shouldUseCredentialsProvider() bool {
	return os.Getenv(credentialsProviderUriEnvVar) != ""
}

func getConfigFromCredentialsProvider(ctx context.Context, forceRefresh bool) (*elasticsearch.Config, error) {
	fmt.Printf("Getting Configuration from credentials provider\n")
	esConfig, err := getBaseConfig()
	if err != nil {
		fmt.Printf("Error getting base config: %v\n", err)
		return nil, err
	}

	key := os.Getenv(credentialsProviderKeyEnvVar)
	mechanism := os.Getenv(credentialsProviderMechanismEnvVar)
	err = setupCredentialsUsingCredentialsProvider(ctx, esConfig, key, mechanism, forceRefresh)
	if err != nil {
		fmt.Printf("Error setting up credentials using credentials provider: %v\n", err)
		return nil, err
	}
	return esConfig, nil
}

// setupCredentialsUsingCredentialsProvider sets up the credentials in the elasticsearch config.
// It returns the updated config.
func setupCredentialsUsingCredentialsProvider(ctx context.Context, esConfig *elasticsearch.Config, key string, mechanism string, forceRefresh bool) error {
	if key == "" {
		fmt.Printf("Error: %s is not set\n", credentialsProviderKeyEnvVar)
		return errCredentialProviderKeyNotSet
	}
	if mechanism == "" {
		fmt.Printf("Error: %s is not set\n", credentialsProviderMechanismEnvVar)
		return errCredentialProviderMechanismNotSet
	}
	if mechanism != apiKeyCredentialsProviderMechanism && mechanism != serviceTokenCredentialsProviderMechanism && mechanism != bearerTokenCredentialsProviderMechanism {
		fmt.Printf("Error: %s is invalid, should be either \"%s\" or \"%s\" or \"%s\"\n", credentialsProviderMechanismEnvVar, apiKeyCredentialsProviderMechanism, serviceTokenCredentialsProviderMechanism, bearerTokenCredentialsProviderMechanism)
		return errCredentialProviderMechanismInvalid
	}

	credential, err := credentials.AcquireCredentials(ctx, key, forceRefresh)
	if err != nil {
		fmt.Printf("Error acquiring credentials: %v\n", err)
		return err
	}

	fmt.Printf("Credential acquired: %s\n", credential)

	if mechanism == apiKeyCredentialsProviderMechanism {
		fmt.Println("Using API key credentials provider mechanism")
		esConfig.APIKey = credential
	} else if mechanism == serviceTokenCredentialsProviderMechanism {
		fmt.Println("Using service token credentials provider mechanism")
		esConfig.ServiceToken = credential
	} else {
		fmt.Println("Using bearer token credentials provider mechanism")
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
		Logger: &estransport.TextLogger{
			Output:             os.Stdout,
			EnableRequestBody:  true,
			EnableResponseBody: true,
		},
		// EnableDebugLogger: true,
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

	certPool, err := loadCACert()
	if err != nil {
		esConfig.Transport = &debugTransport{rt: http.DefaultTransport}
		fmt.Printf("Error loading CA cert pool: %v\n", err)
	} else {
		fmt.Printf("Adding cert pool to transport\n")
		baseTransport := &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: certPool,
			},
		}

		esConfig.Transport = &debugTransport{rt: baseTransport}
	}

	// Read the CA certificate if provided
	caCertPath := os.Getenv("ELASTICSEARCH_CA_CERT_PATH")
	if caCertPath != "" {
		fmt.Printf("Using CA Certificate from path: %s\n", caCertPath)
		cert, err := os.ReadFile(caCertPath)
		if err != nil {
			return nil, fmt.Errorf("error reading CA certificate. Path: %s, Error: %v", caCertPath, err)
		}

		esConfig.CACert = cert
	}

	fmt.Printf("Elasticsearch config: %+v\n", esConfig)

	return &esConfig, nil
}

func loadCACert() (*x509.CertPool, error) {
	caCertPath := os.Getenv("ELASTICSEARCH_CA_CERT_POOL_PATH")
	if caCertPath == "" {
		return nil, fmt.Errorf("CA certificate path is empty")
	}
	certPool := x509.NewCertPool()
	cert, err := os.ReadFile(caCertPath)
	if err != nil {
		return nil, err
	}
	if ok := certPool.AppendCertsFromPEM(cert); !ok {
		return nil, fmt.Errorf("failed to append cert")
	}
	return certPool, nil
}

type debugTransport struct {
	rt http.RoundTripper
}

func (d *debugTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	fmt.Println("<========================= REQUEST")
	fmt.Printf("%s %s\n", req.Method, req.URL)

	for name, values := range req.Header {
		for _, v := range values {
			fmt.Printf("%s: %s\n", name, v)
		}
	}

	if req.Body != nil {
		bodyBytes, _ := io.ReadAll(req.Body)
		req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		if len(bodyBytes) > 0 {
			fmt.Println("Request Body:", string(bodyBytes))
		}
	}

	resp, err := d.rt.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	fmt.Println("============================> RESPONSE")
	fmt.Println("Status:", resp.Status)
	for name, values := range resp.Header {
		for _, v := range values {
			fmt.Printf("%s: %s\n", name, v)
		}
	}

	if resp.Body != nil {
		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		if len(bodyBytes) > 0 {
			fmt.Println("Response Body:", string(bodyBytes))
		}
	}

	return resp, nil
}
