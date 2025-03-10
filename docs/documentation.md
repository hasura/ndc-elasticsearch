# Documentation

## Pagination
Pagination is supported via the `offset` operator and the `search_after` operator.

### `offset`
The `offset` operator can be used if the sum of page size (`limit`) and the `offset` value is less than or equal to the value of [`index.max_result_window`](https://www.elastic.co/guide/en/elasticsearch/reference/current/index-modules.html#index-max-result-window) (defaults to 10,000). This is a restriction imposed by Elasticsearch. For paginating further, please use the `search_after` operator

### `search_after`
The `search_after` operator can be used to page more than [`index.max_result_window`](https://www.elastic.co/guide/en/elasticsearch/reference/current/index-modules.html#index-max-result-window) (default: 10,000) results. The `search_after` operator exposed in GraphQL queries is functionally and syntactically similar to the `search_after` operator in the Elasticsearch. It expects an array of sort values as its argument ([read more about `search_after` in Elasticsearch](https://www.elastic.co/guide/en/elasticsearch/reference/current/paginate-search-results.html#search-after)).

Please note the following requirements for correctly using the `search_after` operator:
1. Any query using the `search_after` operator must also include the `order_by` clause
2. The order of elements in `search_after` should be identical to the order of corresponding fields in `order_by`. For example, consider a model that has got the fields `email` and `customerId` and you want to sort by both. The correct values would be 

```graphql
order_by: [
  {customerId: Asc}, 
  {email: Asc}
], 
args: {
  searchAfter: [
    "cust005", 
    "cust_5@abc.xyz"
  ]
}
```

and, the incorrect way would be 

```graphql
order_by: [
  {customerId: Asc}, 
  {email: Asc}
], 
args: {
  searchAfter: [ // the order of elements is not the same as the order of fields in order_by
    "cust_5@abc.xyz",
    "cust005", 
  ]
}
```

## `/query/explain`

NDC Elasticsearch supports the [`/query/explain` endpoint from the NDC Spec](https://hasura.github.io/ndc-spec/specification/explain.html) using Elasticsearch's [Search Profile API](https://www.elastic.co/guide/en/elasticsearch/reference/current/search-profile.html). Elasticsearch's [Search Explain API](https://www.elastic.co/guide/en/elasticsearch/reference/current/search-explain.html) is not used because it requires a document ID, which is not avaialble at the time of query.