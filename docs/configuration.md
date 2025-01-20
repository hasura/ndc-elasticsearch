# Configuration

## Initializing & Updating a Configuration Directory

The connector requires a configuration directory to run.

### Using the ndc-elasticsearch Executable

If you have the executable:

```bash
ndc-elasticsearch update --configuration=STRING
```
It initialize and updates the configuration directory by fetching mappings from Elasticsearch.

See also: [development instructions](./development.md)

## Index mappings

Index mappings are added by introspecting the Elasticsearch database provided during update of the configuration directory.
These mappings are similar to what we get in [Elasticsearch's mappings API](https://www.elastic.co/guide/en/elasticsearch/reference/current/indices-get-mapping.html).

## Index aliases

Index aliases are added by intropsecting the Elasticsearch database during the update of the configuration directory. Aliases for an index are added as separate indexes in the `indices` key of the `configuration.json` file, with the mappings of the original index copied to them.

More reading on Elasticsearch index aliases: https://www.elastic.co/guide/en/elasticsearch/reference/current/aliases.html

> **NOTE** 
>
> If you change an alias in a way that changes its underlying mappings, please re-introspect the datasource to get the updated mappings for the alias.

## Native Queries

Native Queries allow you to run custom DSL queries on your Elasticsearch. This enables you to run queries that are not supported by Hasura DDN's GraphQL engine. This unlocks the full power of your search-engine, allowing you to run complex queries all directly from your Hasura GraphQL API.

In the `internal` section within the `dsl`, you have the option to write an Elasticsearch DSL query. Additionally, you can create a native query in a `.json` file located in your configuration directory, usually within a specific subdirectory. When specifying the query in the `dsl`, use the `file` option.

Your file may only contain only a single JSON DSL query.

Native Queries can take arguments using the `{{argument_name}}` syntax. Arguments must be specified along with their type.

```json
{
    "query": {
        "range": {
            "age": {
                "gte": "{{gte}}"
            }
        }
    }
}
```

Then add the query to your `configuration.json` file. You'll need to determine the query return type.

The return type can either be of kind `defination` or `index`. kind defination requires a `mappnigs` section in the return type where
you can define custom mappings for your returned documents (Logical Models).

Set `kind` to `index` to use mappings of the existing index in the `indices` section of the configuration.

### Examples:
1. Using `internal` parameter

```json
{
    "aggregate_query": {
        "dsl": {
            "internal": {
                "aggs": {
                    "price_outlier": {
                        "percentiles": {
                            "field": "products.base_price",
                            "percents": [
                                95,
                                99,
                                99.9
                            ]
                        }
                    }
                }
            }
        },
        "index": "kibana_sample_data_ecommerce",
        "return_type": {
            "kind": "index"
        }
    }
}
```

2. Using `file` parameter

```json
{
    "range_query": {
        "dsl": {
            "file": "native_queries/range.json"
        },
        "index": "my_sample_index",
        "return_type": {
            "kind": "defination",
            "mappings": {
                "properties": {
                    "range": {
                        "age": {
                            "type": "integer"
                        }
                    }
                }
            }
        },
        "arguments": {
            "gte": {
                "type": "integer"
            },
            "lte": {
                "type": "integer"
            }
        }
    }
}
```

The CLI provides `validate` command to validate your configuration directory:

```bash
ndc-elasticsearch validate --configuration=STRING
```

### Note on Configuration Structure Changes

During this active development phase, the configuration structure may change.

The CLI also provides an `upgrade` command to upgrade your configuration directory to be compatible with the latest connector version

```bash
ndc-elasticsearch upgrade --dir-from=STRING --dir-to=STRING
