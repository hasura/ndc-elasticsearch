## Credentials Provider

If you have an auth service that can provide credentials to NDC Elasticsearch, you should make use of the credentials provider in the connector. The credentials provider works by requesting credentials from your auth service at the connector startup. The auth service should return a json response with the credentials present as a string in the root level `credentials` key and a `200` response code to be compliant with the credentials provider. Follwing is an example of a compliant response:

```json
{
  "credentials": "my-api-key"
}
```

### Usage

To use credentials provider, only set the `ELASTICSEARCH_URL` env var when using `ddn connector init -i`. After that, once the connector is initialized, set up the following env vars using the command `ddn connector env add $my-connector --env $NEW_VAR=$value`

| Env Var                                      | Description                                                                                                                  |
| -------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------- |
| HASURA_CREDENTIALS_PROVIDER_URI              | The webhook URI for the auth service                                                                                         |
| ELASTICSEARCH_CREDENTIALS_PROVIDER_KEY       | This is the key for the credentials provider                                                                                 |
| ELASTICSEARCH_CREDENTIALS_PROVIDER_MECHANISM | This is the security credential that is expected from the credential provider service. Could be `api-key` or `service-token` or `bearer-token` |
