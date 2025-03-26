## Credentials Provider

If you have an auth service that can provide credentials to NDC Elasticsearch, you should make use of the credentials provider in the connector. To use credentials provider, don't give any auth data to the connector when using `ddn connector init -i`. Instead, once the connector is initialized, set up the following env vars:

| Env Var                                      | Description                                                                                                                  |
| -------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------- |
| HASURA_CREDENTIALS_PROVIDER_URI              | The webhook URI for the auth service                                                                                         |
| ELASTICSEARCH_CREDENTIALS_PROVIDER_KEY       | This is the key for the credentials provider                                                                                 |
| ELASTICSEARCH_CREDENTIALS_PROVIDER_MECHANISM | This is the security credential that is expected from the credential provider service. Could be `api-key` or `service-token` |
