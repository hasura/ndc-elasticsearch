package types

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfigurationGetIndex(t *testing.T) {
	tests := []struct {
		name          string
		configuration string
		indexName     string
		want          string
	}{
		{
			name:          "transaction_payments",
			configuration: configurationTransactions,
			indexName:     "payments",
			want: `{
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
}`,
		},
		{
			name:          "transaction_user_behavior",
			configuration: configurationTransactions,
			indexName:     "user_behavior",
			want: `{
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
}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := getConfiguration(tt.configuration)

			got, err := config.GetIndex(tt.indexName)
			assert.NoError(t, err)

			gotJson, err := json.MarshalIndent(got, "", "  ")
			assert.NoError(t, err)

			assert.JSONEq(t, tt.want, string(gotJson))
		})
	}
}

func TestConfigurationGetFieldMap(t *testing.T) {
	tests := []struct {
		name          string
		configuration string
		indexName     string
		fieldName     string
		want          string
	}{
		{
			name:          "transaction_customers_name",
			configuration: configurationTransactions,
			indexName:     "customers",
			fieldName:     "name",
			want: `{
  "fields": {
    "keyword": {
      "ignore_above": 256,
      "type": "keyword"
    }
  },
  "type": "text"
}`,
		},
		{
			name:          "transaction_payments_paymentStatus",
			configuration: configurationTransactions,
			indexName:     "payments",
			fieldName:     "payment_status",
			want: `{
  "type": "keyword"
}`,
		},
		{
			name:          "transaction_transactions_transactionDetails",
			configuration: configurationTransactions,
			indexName:     "transactions",
			fieldName:     "transaction_details",
			want: `{
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
}`,
		},
		{
			name:          "transaction_transactions_transactionDetails_itemName",
			configuration: configurationTransactions,
			indexName:     "transactions",
			fieldName:     "transaction_details.item_name",
			want: `{
  "fields": {
    "keyword": {
      "ignore_above": 256,
      "type": "keyword"
    }
  },
  "type": "text"
}`,
		},
		{
			name:          "transaction_transactions_transactionDetails_currency",
			configuration: configurationTransactions,
			indexName:     "transactions",
			fieldName:     "transaction_details.currency",
			want: `{
  "type": "keyword"
}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := getConfiguration(tt.configuration)

			got, err := config.GetFieldMap(tt.indexName, tt.fieldName)
			assert.NoError(t, err)

			gotJson, err := json.MarshalIndent(got, "", "  ")
			assert.NoError(t, err)
			fmt.Println(string(gotJson))
			assert.JSONEq(t, tt.want, string(gotJson))
		})
	}
}

func TestConfigurationGetFieldProperties(t *testing.T) {
	tests := []struct {
		name                 string
		configuration        string
		indexName            string
		fieldName            string
		wantFieldType        string
		wantSubtypes         []string
		wantFieldDataEnabled bool
	}{
		{
			name:                 "transaction_customers_name",
			configuration:        configurationTransactions,
			indexName:            "customers",
			fieldName:            "name",
			wantFieldType:        "text",
			wantSubtypes:         []string{"keyword"},
			wantFieldDataEnabled: false,
		},
		{
			name:                 "transaction_logs_log_level",
			configuration:        configurationTransactions,
			indexName:            "logs",
			fieldName:            "log_level",
			wantFieldType:        "keyword",
			wantSubtypes:         []string{},
			wantFieldDataEnabled: false,
		},
		{
			name:                 "transaction_transactions_transactionDetails_itemName",
			configuration:        configurationTransactions,
			indexName:            "transactions",
			fieldName:            "transaction_details.item_name",
			wantFieldType:        "text",
			wantSubtypes:         []string{"keyword"},
			wantFieldDataEnabled: false,
		},
		{
			name:                 "transaction_user_behavior_actions",
			configuration:        configurationTransactions,
			indexName:            "user_behavior",
			fieldName:            "actions",
			wantFieldType:        "nested",
			wantSubtypes:         []string{},
			wantFieldDataEnabled: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := getConfiguration(tt.configuration)

			gotFieldType, gotFieldSubTypes, gotFieldDataEnabled, err := config.GetFieldProperties(tt.indexName, tt.fieldName)
			assert.NoError(t, err)

			assert.Equal(t, tt.wantFieldType, gotFieldType)
			assert.Equal(t, tt.wantSubtypes, gotFieldSubTypes)
			assert.Equal(t, tt.wantFieldDataEnabled, gotFieldDataEnabled)
		})
	}
}

func getConfiguration(configurationStr string) *Configuration {
	var configuration Configuration
	err := json.Unmarshal([]byte(configurationStr), &configuration)
	if err != nil {
		panic(err)
	}
	return &configuration
}

const configurationTransactions = `{
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