package connector

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"testing"

	"github.com/hasura/ndc-elasticsearch/types"
	"github.com/hasura/ndc-sdk-go/schema"
	"github.com/stretchr/testify/assert"
)

func TestPrepareElasticsearchQuery(t *testing.T) {
	tests := []struct {
		name          string
		ndcRequest    string
		expectedQuery string
	}{
		{
			name:          "Simple_Query_001",
			ndcRequest:    ndcRequest_001,
			expectedQuery: expectedQuery_001,
		},
		{
			name:          "Simple_Query_With_Limit_002",
			ndcRequest:    ndcRequest_002,
			expectedQuery: expectedQuery_002,
		},
		{
			name:          "Nested_Query_003",
			ndcRequest:    ndcRequest_003,
			expectedQuery: expectedQuery_003,
		},
		{
			name:          "Nested_Query_With_Limit_004",
			ndcRequest:    ndcRequest_004,
			expectedQuery: expectedQuery_004,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			state := &types.State{}

			ctx = context.WithValue(ctx, "postProcessor", &types.PostProcessor{})

			var request schema.QueryRequest
			err := json.Unmarshal([]byte(tt.ndcRequest), &request)
			assert.NoError(t, err)

			query, err := prepareElasticsearchQuery(ctx, &request, state, request.Collection)
			assert.NoError(t, err)

			// this correction is added because sometimes the order of _source array would change which resulted in the tests being flaky
			query, err = sortSourceArray(query)
			assert.NoError(t, err)

			queryJson, err := json.MarshalIndent(query, "", "  ")
			assert.NoError(t, err)

			assert.JSONEq(t, tt.expectedQuery, string(queryJson))
		})
	}
}

// A helper function to sort the _source array in the query
//
// Required because the order of the _source array in the query is not fixed, and the tests were flaky due to this
func sortSourceArray(query map[string]interface{}) (map[string]interface{}, error) {
	source, ok := query["_source"].([]string)
	if !ok {
		return nil, fmt.Errorf("expected _source to be of type []string, got %T", query["_source"])
	}

	sort.Strings(source)
	query["_source"] = source
	return query, nil
}

const ndcRequest_001 = `{
  "arguments": {},
  "collection": "customers",
  "collection_relationships": {},
  "query": {
    "fields": {
      "id": {
        "column": "_id",
        "type": "column"
      },
      "name": {
        "column": "name",
        "type": "column"
      }
    }
  }
}`

const expectedQuery_001 = `{
  "_source": [
    "_id",
    "name"
  ],
  "size": 10000
}`

const ndcRequest_002 = `{
  "arguments": {},
  "collection": "customers",
  "collection_relationships": {},
  "query": {
    "fields": {
      "id": {
        "column": "_id",
        "type": "column"
      },
      "name": {
        "column": "name",
        "type": "column"
      }
    },
    "limit": 500
  }
}`

const expectedQuery_002 = `{
  "_source": [
    "_id", 
	"name"
  ],
  "size": 500
}`

const ndcRequest_003 = `{
  "arguments": {},
  "collection": "transactions",
  "collection_relationships": {},
  "query": {
    "fields": {
      "customerId": {
        "column": "customer_id",
        "type": "column"
      },
      "id": {
        "column": "_id",
        "type": "column"
      },
      "timestamp": {
        "column": "timestamp",
        "type": "column"
      },
      "transactionDetails": {
        "column": "transaction_details",
        "fields": {
          "fields": {
            "fields": {
              "currency": {
                "column": "currency",
                "type": "column"
              },
              "itemId": {
                "column": "item_id",
                "type": "column"
              },
              "itemName": {
                "column": "item_name",
                "type": "column"
              },
              "price": {
                "column": "price",
                "type": "column"
              },
              "quantity": {
                "column": "quantity",
                "type": "column"
              }
            },
            "type": "object"
          },
          "type": "array"
        },
        "type": "column"
      },
      "transactionId": {
        "column": "transaction_id",
        "type": "column"
      }
    }
  }
}`

const expectedQuery_003 = `{
  "_source": [
    "_id", 
	"customer_id", 
	"timestamp", 
	"transaction_details.currency", 
	"transaction_details.item_id", 
	"transaction_details.item_name", 
	"transaction_details.price", 
	"transaction_details.quantity", 
	"transaction_id"
  ],
  "size": 10000
}`

const ndcRequest_004 = `{
  "arguments": {},
  "collection": "transactions",
  "collection_relationships": {},
  "query": {
    "fields": {
      "customerId": {
        "column": "customer_id",
        "type": "column"
      },
      "id": {
        "column": "_id",
        "type": "column"
      },
      "timestamp": {
        "column": "timestamp",
        "type": "column"
      },
      "transactionDetails": {
        "column": "transaction_details",
        "fields": {
          "fields": {
            "fields": {
              "currency": {
                "column": "currency",
                "type": "column"
              },
              "itemId": {
                "column": "item_id",
                "type": "column"
              },
              "itemName": {
                "column": "item_name",
                "type": "column"
              },
              "price": {
                "column": "price",
                "type": "column"
              },
              "quantity": {
                "column": "quantity",
                "type": "column"
              }
            },
            "type": "object"
          },
          "type": "array"
        },
        "type": "column"
      },
      "transactionId": {
        "column": "transaction_id",
        "type": "column"
      }
    },
    "limit": 20
  }
}`

const expectedQuery_004 = `{
  "_source": [
    "_id", 
	"customer_id", 
	"timestamp", 
	"transaction_details.currency", 
	"transaction_details.item_id", 
	"transaction_details.item_name", 
	"transaction_details.price", 
	"transaction_details.quantity", 
	"transaction_id"
  ],
  "size": 20
}`
