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
		name             string
		ndcRequest       string
		expectedQuery    string
		configurationStr string
	}{
		{
			name:             "Simple_Query_001",
			ndcRequest:       ndcRequest_001,
			expectedQuery:    expectedQuery_001,
			configurationStr: configuration_payments_001,
		},
		{
			name:             "Simple_Query_With_Limit_002",
			ndcRequest:       ndcRequest_002,
			expectedQuery:    expectedQuery_002,
			configurationStr: configuration_payments_001,
		},
		{
			name:             "Nested_Query_003",
			ndcRequest:       ndcRequest_003,
			expectedQuery:    expectedQuery_003,
			configurationStr: configuration_payments_001,
		},
		{
			name:             "Nested_Query_With_Limit_004",
			ndcRequest:       ndcRequest_004,
			expectedQuery:    expectedQuery_004,
			configurationStr: configuration_payments_001,
		},
		{
			name:             "Sort_by_Type",
			ndcRequest:       ndcRequest_sort_001,
			expectedQuery:    expectedQuery_sort_001,
			configurationStr: configuration_payments_001,
		},
		{
			name:             "Sort_by_Subtype",
			ndcRequest:       ndcRequest_sort_002,
			expectedQuery:    expectedQuery_sort_002,
			configurationStr: configuration_payments_001,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			state := &types.State{
				Client:                   nil,
				SupportedSortFields:      make(map[string]interface{}),
				SupportedAggregateFields: make(map[string]interface{}),
				SupportedFilterFields:    make(map[string]interface{}),
				ElasticsearchInfo:        make(map[string]interface{}),
				NestedFields:             make(map[string]interface{}),
				Schema:                   nil, // Assuming Tracer is an interface, set to nil or an empty implementation
			}

			configuration := getConfiguration(tt.configurationStr)

			ParseConfigurationToSchema(configuration, state)

			ctx = context.WithValue(ctx, "postProcessor", &types.PostProcessor{})

			var request schema.QueryRequest
			err := json.Unmarshal([]byte(tt.ndcRequest), &request)
			assert.NoError(t, err)

			query, err := prepareElasticsearchQuery(ctx, &request, state, request.Collection, configuration)
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

func getConfiguration(configurationStr string) *types.Configuration {
	var configuration types.Configuration
	err := json.Unmarshal([]byte(configurationStr), &configuration)
	if err != nil {
		panic(err)
	}
	return &configuration
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

const ndcRequest_sort_001 = `{
  "arguments": {},
  "collection": "customers",
  "collection_relationships": {},
  "query": {
    "fields": {
      "name": {
        "column": "name",
        "type": "column"
      }
    },
    "order_by": {
      "elements": [
        {
          "order_direction": "asc",
          "target": {
            "name": "email",
            "path": [],
            "type": "column"
          }
        }
      ]
    }
  }
}`

const expectedQuery_sort_001 = `{
  "_source": [
    "name"
  ],
  "size": 10000,
  "sort": [
    {
      "email": {
        "order": "asc"
      }
    }
  ]
}`

const ndcRequest_sort_002 = `{
  "arguments": {},
  "collection": "customers",
  "collection_relationships": {},
  "query": {
    "fields": {
      "name": {
        "column": "name",
        "type": "column"
      }
    },
    "order_by": {
      "elements": [
        {
          "order_direction": "asc",
          "target": {
            "name": "name",
            "path": [],
            "type": "column"
          }
        }
      ]
    }
  }
}`

const expectedQuery_sort_002 = `{
  "_source": [
    "name"
  ],
  "size": 10000,
  "sort": [
    {
      "name.keyword": {
        "order": "asc"
      }
    }
  ]
}`

const configuration_payments_001 = `{
  "indices": {
    "customers": {
      "mappings": {
        "properties": {
          "customer_id": {
            "type": "keyword"
          },
          "email": {
            "type": "keyword"
          },
          "location": {
            "type": "geo_point"
          },
          "name": {
            "fields": {
              "keyword": {
                "ignore_above": 256,
                "type": "keyword"
              }
            },
            "type": "text"
          }
        }
      }
    },
    "logs": {
      "mappings": {
        "properties": {
          "application": {
            "type": "keyword"
          },
          "log_level": {
            "type": "keyword"
          },
          "message": {
            "type": "text"
          },
          "timestamp": {
            "type": "date"
          }
        }
      }
    },
    "metrics": {
      "mappings": {
        "properties": {
          "metric_type": {
            "type": "keyword"
          },
          "metric_unit": {
            "type": "keyword"
          },
          "metric_value": {
            "type": "float"
          },
          "timestamp": {
            "type": "date"
          }
        }
      }
    },
    "payments": {
      "mappings": {
        "properties": {
          "payment_method": {
            "type": "keyword"
          },
          "payment_status": {
            "type": "keyword"
          },
          "transaction_id": {
            "type": "keyword"
          }
        }
      }
    },
    "transactions": {
      "mappings": {
        "properties": {
          "customer_id": {
            "type": "keyword"
          },
          "timestamp": {
            "type": "date"
          },
          "transaction_details": {
            "properties": {
              "currency": {
                "type": "keyword"
              },
              "item_id": {
                "type": "keyword"
              },
              "item_name": {
                "fields": {
                  "keyword": {
                    "ignore_above": 256,
                    "type": "keyword"
                  }
                },
                "type": "text"
              },
              "price": {
                "type": "float"
              },
              "quantity": {
                "type": "integer"
              }
            }
          },
          "transaction_id": {
            "type": "keyword"
          }
        }
      }
    },
    "user_behavior": {
      "mappings": {
        "properties": {
          "actions": {
            "properties": {
              "action_time": {
                "type": "date"
              },
              "action_type": {
                "type": "keyword"
              },
              "metadata": {
                "type": "text"
              }
            },
            "type": "nested"
          },
          "customer_id": {
            "type": "keyword"
          },
          "session_id": {
            "type": "keyword"
          }
        }
      }
    }
  },
  "queries": {}
}`
