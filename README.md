# Elasticsearch Connector

<a href="https://hasura.io/"><img src="./docs/logo.png" align="right" width="200"></a>

[![Docs](https://img.shields.io/badge/docs-v3.x-brightgreen.svg?style=flat)](https://hasura.io/docs/3.0)
[![ndc-hub](https://img.shields.io/badge/ndc--hub-elasticsearch-blue.svg?style=flat)](https://hasura.io/connectors/elasticsearch)
[![License](https://img.shields.io/badge/license-Apache--2.0-purple.svg?style=flat)](LICENSE.txt)

With this connector, Hasura allows you to instantly create a real-time GraphQL API on top of your documents in Elasticsearch. This connector supports Elasticsearch functionalities listed in the table below, allowing for efficient and scalable data operations. Additionally, you will benefit from all the powerful features of Hasura’s Data Delivery Network (DDN) platform, including query pushdown capabilities that delegate all query operations to the Elasticsearch, thereby enhancing query optimization and performance.

This connector is built using the [Go Data Connector SDK](https://github.com/hasura/ndc-sdk-go) and implements the [Data Connector Spec](https://github.com/hasura/ndc-spec).

- [See the listing in the Hasura Hub](https://hasura.io/connectors/elasticsearch)
- [Hasura DDN Documentation](https://hasura.io/docs/3.0)
- [Hasura DDN Quickstart with Elasticsearch](https://hasura.io/docs/3.0/how-to-build-with-ddn/with-elasticsearch)
- [GraphQL on Elasticsearch](https://hasura.io/graphql/database/elasticsearch)

Docs for the Elasticsearch data connector:

- [Architecture](./docs/architecture.md)
- [Code of Conduct](./docs/code-of-conduct.md)
- [Contributing](./docs/contributing.md)
- [Configuration](./docs/configuration.md)
- [Development](./docs/development.md)
- [Security](./docs/security.md)
- [Support](./docs/support.md)

## Features

Below, you'll find a matrix of all supported features for the Elasticsearch connector:

| Feature                                 | Supported |
| --------------------------------------- | --------- |
| Native Queries                          | ❌        |
| Native Mutations                        | ❌        |
| Filter / Search via term                | ✅        |
| Filter / Search via terms               | ✅        |
| Filter / Search via match               | ✅        |
| Filter / Search via match_bool_prefix   | ✅        |
| Filter / Search via match_phrase        | ✅        |
| Filter / Search via prefix              | ✅        |
| Filter / Search via range               | ✅        |
| Filter / Search via regexp              | ✅        |
| Filter / Search via wildcard            | ✅        |
| Filter / Search via terms_set           | ❌        |
| Filter / Search via intervals           | ❌        |
| Filter / Search via query_string        | ❌        |
| Filter / Search via simple_query_string | ❌        |
| Filter / Search via fuzzy               | ❌        |
| Simple Aggregation                      | ✅        |
| Sort                                    | ✅        |
| Paginate via offset                     | ✅        |
| Paginate via search_after               | ✅        |
| Distinct                                | ❌        |
| Enums                                   | ❌        |
| Default Values                          | ✅        |
| User-defined Functions                  | ❌        |
| Index Aliases                           | ✅        |
| Field Aliases                           | ❌        |
| Multi Fields                            | ❌        |
| Runtime Fields                          | ❌        |
| Field Analyzers                         | ❌        |

## Getting Started

Please see [Getting Started with Elasticsearch on Hasura DDN](https://hasura.io/docs/3.0/how-to-build-with-ddn/with-elasticsearch) to instantly get a GraphQL API on Elasticsearch by connecting it to Hasura DDN.

## Detailed Documentation

Please checkout out the [detailed documentation](./docs/documentation.md).

## Contributing

Check out our [contributing guide](./docs/contributing.md) for more details.

## License

The Elasticsearch connector is available under the [Apache License 2.0](https://www.apache.org/licenses/LICENSE-2.0).
