package connector_test

import (
	"encoding/json"
	// "fmt"
	"testing"

	"github.com/hasura/ndc-elasticsearch/types"
	"github.com/stretchr/testify/assert"
	"github.com/hasura/ndc-elasticsearch/connector"
)

const CONFIGURATION_IDENTIFICATION = `{
  "indices": {
    "indentification": {
      "mappings": {
        "properties": {
          "address": {
            "properties": {
              "city": {
                "type": "text"
              },
              "zip": {
                "type": "integer"
              }
            }
          },
          "age": {
            "type": "integer"
          },
          "name": {
            "type": "keyword"
          }
        }
      }
    }
  },
  "queries": {}
}`

const SCHEMA_IDENTIFICATION = `{
  "collections": [
    {
      "arguments": {},
      "foreign_keys": {},
      "name": "indentification",
      "type": "indentification",
      "uniqueness_constraints": {}
    }
  ],
  "functions": [],
  "object_types": {
    "address": {
      "fields": {
        "city": {
          "type": {
            "name": "text",
            "type": "named"
          }
        },
        "zip": {
          "type": {
            "name": "integer",
            "type": "named"
          }
        }
      }
    },
    "indentification": {
      "fields": {
        "_id": {
          "type": {
            "name": "_id",
            "type": "named"
          }
        },
        "address": {
          "type": {
            "element_type": {
              "name": "address",
              "type": "named"
            },
            "type": "array"
          }
        },
        "age": {
          "type": {
            "name": "integer",
            "type": "named"
          }
        },
        "name": {
          "type": {
            "name": "keyword",
            "type": "named"
          }
        }
      }
    },
    "range": {
      "fields": {
        "boost": {
          "description": "(Optional, float) Floating point number used to decrease or increase the relevance scores of a query. Defaults to 1.0.",
          "type": {
            "name": "float",
            "type": "named"
          }
        },
        "gt": {
          "description": "(Optional) Greater than.",
          "type": {
            "name": "double",
            "type": "named"
          }
        },
        "gte": {
          "description": "(Optional) Greater than or equal.",
          "type": {
            "name": "double",
            "type": "named"
          }
        },
        "lt": {
          "description": "(Optional) Less than.",
          "type": {
            "name": "double",
            "type": "named"
          }
        },
        "lte": {
          "description": "(Optional) Less than or equal.",
          "type": {
            "name": "double",
            "type": "named"
          }
        }
      }
    },
    "stats": {
      "fields": {
        "avg": {
          "type": {
            "name": "double",
            "type": "named"
          }
        },
        "count": {
          "type": {
            "name": "integer",
            "type": "named"
          }
        },
        "max": {
          "type": {
            "name": "double",
            "type": "named"
          }
        },
        "min": {
          "type": {
            "name": "double",
            "type": "named"
          }
        },
        "sum": {
          "type": {
            "name": "double",
            "type": "named"
          }
        }
      }
    },
    "string_stats": {
      "fields": {
        "avg_length": {
          "type": {
            "name": "double",
            "type": "named"
          }
        },
        "count": {
          "type": {
            "name": "integer",
            "type": "named"
          }
        },
        "entropy": {
          "type": {
            "name": "double",
            "type": "named"
          }
        },
        "max_length": {
          "type": {
            "name": "integer",
            "type": "named"
          }
        },
        "min_length": {
          "type": {
            "name": "integer",
            "type": "named"
          }
        }
      }
    }
  },
  "procedures": [],
  "scalar_types": {
    "_id": {
      "aggregate_functions": {},
      "comparison_operators": {
        "match": {
          "argument_type": {
            "name": "_id",
            "type": "named"
          },
          "type": "custom"
        },
        "match_phrase": {
          "argument_type": {
            "name": "_id",
            "type": "named"
          },
          "type": "custom"
        },
        "term": {
          "type": "equal"
        },
        "terms": {
          "argument_type": {
            "element_type": {
              "name": "_id",
              "type": "named"
            },
            "type": "array"
          },
          "type": "custom"
        }
      },
      "representation": {
        "type": "string"
      }
    },
    "double": {
      "aggregate_functions": {
        "avg": {
          "result_type": {
            "name": "double",
            "type": "named"
          }
        },
        "cardinality": {
          "result_type": {
            "name": "integer",
            "type": "named"
          }
        },
        "max": {
          "result_type": {
            "name": "double",
            "type": "named"
          }
        },
        "min": {
          "result_type": {
            "name": "double",
            "type": "named"
          }
        },
        "stats": {
          "result_type": {
            "name": "stats",
            "type": "named"
          }
        },
        "sum": {
          "result_type": {
            "name": "double",
            "type": "named"
          }
        },
        "value_count": {
          "result_type": {
            "name": "integer",
            "type": "named"
          }
        }
      },
      "comparison_operators": {
        "match": {
          "argument_type": {
            "name": "double",
            "type": "named"
          },
          "type": "custom"
        },
        "match_phrase": {
          "argument_type": {
            "name": "double",
            "type": "named"
          },
          "type": "custom"
        },
        "term": {
          "type": "equal"
        },
        "terms": {
          "argument_type": {
            "element_type": {
              "name": "double",
              "type": "named"
            },
            "type": "array"
          },
          "type": "custom"
        }
      },
      "representation": {
        "type": "number"
      }
    },
    "float": {
      "aggregate_functions": {
        "avg": {
          "result_type": {
            "name": "float",
            "type": "named"
          }
        },
        "cardinality": {
          "result_type": {
            "name": "integer",
            "type": "named"
          }
        },
        "max": {
          "result_type": {
            "name": "float",
            "type": "named"
          }
        },
        "min": {
          "result_type": {
            "name": "float",
            "type": "named"
          }
        },
        "stats": {
          "result_type": {
            "name": "stats",
            "type": "named"
          }
        },
        "sum": {
          "result_type": {
            "name": "float",
            "type": "named"
          }
        },
        "value_count": {
          "result_type": {
            "name": "integer",
            "type": "named"
          }
        }
      },
      "comparison_operators": {
        "match": {
          "argument_type": {
            "name": "float",
            "type": "named"
          },
          "type": "custom"
        },
        "match_phrase": {
          "argument_type": {
            "name": "float",
            "type": "named"
          },
          "type": "custom"
        },
        "term": {
          "type": "equal"
        },
        "terms": {
          "argument_type": {
            "element_type": {
              "name": "float",
              "type": "named"
            },
            "type": "array"
          },
          "type": "custom"
        }
      },
      "representation": {
        "type": "number"
      }
    },
    "integer": {
      "aggregate_functions": {
        "avg": {
          "result_type": {
            "name": "integer",
            "type": "named"
          }
        },
        "cardinality": {
          "result_type": {
            "name": "integer",
            "type": "named"
          }
        },
        "max": {
          "result_type": {
            "name": "integer",
            "type": "named"
          }
        },
        "min": {
          "result_type": {
            "name": "integer",
            "type": "named"
          }
        },
        "stats": {
          "result_type": {
            "name": "stats",
            "type": "named"
          }
        },
        "sum": {
          "result_type": {
            "name": "integer",
            "type": "named"
          }
        },
        "value_count": {
          "result_type": {
            "name": "integer",
            "type": "named"
          }
        }
      },
      "comparison_operators": {
        "match": {
          "argument_type": {
            "name": "integer",
            "type": "named"
          },
          "type": "custom"
        },
        "match_phrase": {
          "argument_type": {
            "name": "integer",
            "type": "named"
          },
          "type": "custom"
        },
        "term": {
          "type": "equal"
        },
        "terms": {
          "argument_type": {
            "element_type": {
              "name": "integer",
              "type": "named"
            },
            "type": "array"
          },
          "type": "custom"
        }
      },
      "representation": {
        "type": "integer"
      }
    },
    "keyword": {
      "aggregate_functions": {
        "cardinality": {
          "result_type": {
            "name": "integer",
            "type": "named"
          }
        },
        "string_stats": {
          "result_type": {
            "name": "string_stats",
            "type": "named"
          }
        },
        "value_count": {
          "result_type": {
            "name": "integer",
            "type": "named"
          }
        }
      },
      "comparison_operators": {
        "match": {
          "argument_type": {
            "name": "keyword",
            "type": "named"
          },
          "type": "custom"
        },
        "match_bool_prefix": {
          "argument_type": {
            "name": "keyword",
            "type": "named"
          },
          "type": "custom"
        },
        "match_phrase": {
          "argument_type": {
            "name": "keyword",
            "type": "named"
          },
          "type": "custom"
        },
        "prefix": {
          "argument_type": {
            "name": "keyword",
            "type": "named"
          },
          "type": "custom"
        },
        "regexp": {
          "argument_type": {
            "name": "keyword",
            "type": "named"
          },
          "type": "custom"
        },
        "term": {
          "type": "equal"
        },
        "terms": {
          "argument_type": {
            "element_type": {
              "name": "keyword",
              "type": "named"
            },
            "type": "array"
          },
          "type": "custom"
        },
        "wildcard": {
          "argument_type": {
            "name": "keyword",
            "type": "named"
          },
          "type": "custom"
        }
      },
      "representation": {
        "type": "string"
      }
    },
    "long": {
      "aggregate_functions": {
        "avg": {
          "result_type": {
            "name": "long",
            "type": "named"
          }
        },
        "cardinality": {
          "result_type": {
            "name": "integer",
            "type": "named"
          }
        },
        "max": {
          "result_type": {
            "name": "long",
            "type": "named"
          }
        },
        "min": {
          "result_type": {
            "name": "long",
            "type": "named"
          }
        },
        "stats": {
          "result_type": {
            "name": "stats",
            "type": "named"
          }
        },
        "sum": {
          "result_type": {
            "name": "long",
            "type": "named"
          }
        },
        "value_count": {
          "result_type": {
            "name": "integer",
            "type": "named"
          }
        }
      },
      "comparison_operators": {
        "match": {
          "argument_type": {
            "name": "long",
            "type": "named"
          },
          "type": "custom"
        },
        "match_phrase": {
          "argument_type": {
            "name": "long",
            "type": "named"
          },
          "type": "custom"
        },
        "term": {
          "type": "equal"
        },
        "terms": {
          "argument_type": {
            "element_type": {
              "name": "long",
              "type": "named"
            },
            "type": "array"
          },
          "type": "custom"
        }
      },
      "representation": {
        "type": "integer"
      }
    },
    "text": {
      "aggregate_functions": {},
      "comparison_operators": {
        "match": {
          "argument_type": {
            "name": "text",
            "type": "named"
          },
          "type": "custom"
        },
        "match_bool_prefix": {
          "argument_type": {
            "name": "text",
            "type": "named"
          },
          "type": "custom"
        },
        "match_phrase": {
          "argument_type": {
            "name": "text",
            "type": "named"
          },
          "type": "custom"
        },
        "match_phrase_prefix": {
          "argument_type": {
            "name": "text",
            "type": "named"
          },
          "type": "custom"
        },
        "prefix": {
          "argument_type": {
            "name": "text",
            "type": "named"
          },
          "type": "custom"
        },
        "regexp": {
          "argument_type": {
            "name": "text",
            "type": "named"
          },
          "type": "custom"
        },
        "term": {
          "type": "equal"
        },
        "terms": {
          "argument_type": {
            "element_type": {
              "name": "text",
              "type": "named"
            },
            "type": "array"
          },
          "type": "custom"
        },
        "wildcard": {
          "argument_type": {
            "name": "text",
            "type": "named"
          },
          "type": "custom"
        }
      },
      "representation": {
        "type": "string"
      }
    }
  }
}`

const CONFIGURATION_BOOKS = `{
  "indices": {
    "my_book_index": {
      "mappings": {
        "properties": {
          "author": {
            "type": "keyword"
          },
          "description": {
            "type": "text"
          },
          "genre": {
            "type": "keyword"
          },
          "pages": {
            "type": "integer"
          },
          "published_date": {
            "format": "yyyy-MM-dd",
            "type": "date"
          },
          "rating": {
            "type": "float"
          },
          "title": {
            "fields": {
              "keyword": {
                "type": "keyword"
              }
            },
            "type": "text"
          }
        }
      }
    }
  },
  "queries": {}
}`

const SCHEMA_BOOKS = `{
  "collections": [
    {
      "arguments": {},
      "foreign_keys": {},
      "name": "my_book_index",
      "type": "my_book_index",
      "uniqueness_constraints": {}
    }
  ],
  "functions": [],
  "object_types": {
    "my_book_index": {
      "fields": {
        "_id": {
          "type": {
            "name": "_id",
            "type": "named"
          }
        },
        "author": {
          "type": {
            "name": "keyword",
            "type": "named"
          }
        },
        "description": {
          "type": {
            "name": "text",
            "type": "named"
          }
        },
        "genre": {
          "type": {
            "name": "keyword",
            "type": "named"
          }
        },
        "pages": {
          "type": {
            "name": "integer",
            "type": "named"
          }
        },
        "published_date": {
          "type": {
            "name": "date",
            "type": "named"
          }
        },
        "rating": {
          "type": {
            "name": "float",
            "type": "named"
          }
        },
        "title": {
          "type": {
            "name": "text.keyword",
            "type": "named"
          }
        }
      }
    },
    "range": {
      "fields": {
        "boost": {
          "description": "(Optional, float) Floating point number used to decrease or increase the relevance scores of a query. Defaults to 1.0.",
          "type": {
            "name": "float",
            "type": "named"
          }
        },
        "gt": {
          "description": "(Optional) Greater than.",
          "type": {
            "name": "double",
            "type": "named"
          }
        },
        "gte": {
          "description": "(Optional) Greater than or equal.",
          "type": {
            "name": "double",
            "type": "named"
          }
        },
        "lt": {
          "description": "(Optional) Less than.",
          "type": {
            "name": "double",
            "type": "named"
          }
        },
        "lte": {
          "description": "(Optional) Less than or equal.",
          "type": {
            "name": "double",
            "type": "named"
          }
        }
      }
    },
    "stats": {
      "fields": {
        "avg": {
          "type": {
            "name": "double",
            "type": "named"
          }
        },
        "count": {
          "type": {
            "name": "integer",
            "type": "named"
          }
        },
        "max": {
          "type": {
            "name": "double",
            "type": "named"
          }
        },
        "min": {
          "type": {
            "name": "double",
            "type": "named"
          }
        },
        "sum": {
          "type": {
            "name": "double",
            "type": "named"
          }
        }
      }
    },
    "string_stats": {
      "fields": {
        "avg_length": {
          "type": {
            "name": "double",
            "type": "named"
          }
        },
        "count": {
          "type": {
            "name": "integer",
            "type": "named"
          }
        },
        "entropy": {
          "type": {
            "name": "double",
            "type": "named"
          }
        },
        "max_length": {
          "type": {
            "name": "integer",
            "type": "named"
          }
        },
        "min_length": {
          "type": {
            "name": "integer",
            "type": "named"
          }
        }
      }
    }
  },
  "procedures": [],
  "scalar_types": {
    "_id": {
      "aggregate_functions": {},
      "comparison_operators": {
        "match": {
          "argument_type": {
            "name": "_id",
            "type": "named"
          },
          "type": "custom"
        },
        "match_phrase": {
          "argument_type": {
            "name": "_id",
            "type": "named"
          },
          "type": "custom"
        },
        "term": {
          "type": "equal"
        },
        "terms": {
          "argument_type": {
            "element_type": {
              "name": "_id",
              "type": "named"
            },
            "type": "array"
          },
          "type": "custom"
        }
      },
      "representation": {
        "type": "string"
      }
    },
    "date": {
      "aggregate_functions": {
        "avg": {
          "result_type": {
            "name": "long",
            "type": "named"
          }
        },
        "cardinality": {
          "result_type": {
            "name": "integer",
            "type": "named"
          }
        },
        "max": {
          "result_type": {
            "name": "long",
            "type": "named"
          }
        },
        "min": {
          "result_type": {
            "name": "long",
            "type": "named"
          }
        },
        "stats": {
          "result_type": {
            "name": "stats",
            "type": "named"
          }
        },
        "sum": {
          "result_type": {
            "name": "long",
            "type": "named"
          }
        },
        "value_count": {
          "result_type": {
            "name": "integer",
            "type": "named"
          }
        }
      },
      "comparison_operators": {
        "match": {
          "argument_type": {
            "name": "date",
            "type": "named"
          },
          "type": "custom"
        },
        "match_phrase": {
          "argument_type": {
            "name": "date",
            "type": "named"
          },
          "type": "custom"
        },
        "term": {
          "type": "equal"
        },
        "terms": {
          "argument_type": {
            "element_type": {
              "name": "date",
              "type": "named"
            },
            "type": "array"
          },
          "type": "custom"
        }
      },
      "representation": {
        "type": "string"
      }
    },
    "double": {
      "aggregate_functions": {
        "avg": {
          "result_type": {
            "name": "double",
            "type": "named"
          }
        },
        "cardinality": {
          "result_type": {
            "name": "integer",
            "type": "named"
          }
        },
        "max": {
          "result_type": {
            "name": "double",
            "type": "named"
          }
        },
        "min": {
          "result_type": {
            "name": "double",
            "type": "named"
          }
        },
        "stats": {
          "result_type": {
            "name": "stats",
            "type": "named"
          }
        },
        "sum": {
          "result_type": {
            "name": "double",
            "type": "named"
          }
        },
        "value_count": {
          "result_type": {
            "name": "integer",
            "type": "named"
          }
        }
      },
      "comparison_operators": {
        "match": {
          "argument_type": {
            "name": "double",
            "type": "named"
          },
          "type": "custom"
        },
        "match_phrase": {
          "argument_type": {
            "name": "double",
            "type": "named"
          },
          "type": "custom"
        },
        "term": {
          "type": "equal"
        },
        "terms": {
          "argument_type": {
            "element_type": {
              "name": "double",
              "type": "named"
            },
            "type": "array"
          },
          "type": "custom"
        }
      },
      "representation": {
        "type": "number"
      }
    },
    "float": {
      "aggregate_functions": {
        "avg": {
          "result_type": {
            "name": "float",
            "type": "named"
          }
        },
        "cardinality": {
          "result_type": {
            "name": "integer",
            "type": "named"
          }
        },
        "max": {
          "result_type": {
            "name": "float",
            "type": "named"
          }
        },
        "min": {
          "result_type": {
            "name": "float",
            "type": "named"
          }
        },
        "stats": {
          "result_type": {
            "name": "stats",
            "type": "named"
          }
        },
        "sum": {
          "result_type": {
            "name": "float",
            "type": "named"
          }
        },
        "value_count": {
          "result_type": {
            "name": "integer",
            "type": "named"
          }
        }
      },
      "comparison_operators": {
        "match": {
          "argument_type": {
            "name": "float",
            "type": "named"
          },
          "type": "custom"
        },
        "match_phrase": {
          "argument_type": {
            "name": "float",
            "type": "named"
          },
          "type": "custom"
        },
        "term": {
          "type": "equal"
        },
        "terms": {
          "argument_type": {
            "element_type": {
              "name": "float",
              "type": "named"
            },
            "type": "array"
          },
          "type": "custom"
        }
      },
      "representation": {
        "type": "number"
      }
    },
    "integer": {
      "aggregate_functions": {
        "avg": {
          "result_type": {
            "name": "integer",
            "type": "named"
          }
        },
        "cardinality": {
          "result_type": {
            "name": "integer",
            "type": "named"
          }
        },
        "max": {
          "result_type": {
            "name": "integer",
            "type": "named"
          }
        },
        "min": {
          "result_type": {
            "name": "integer",
            "type": "named"
          }
        },
        "stats": {
          "result_type": {
            "name": "stats",
            "type": "named"
          }
        },
        "sum": {
          "result_type": {
            "name": "integer",
            "type": "named"
          }
        },
        "value_count": {
          "result_type": {
            "name": "integer",
            "type": "named"
          }
        }
      },
      "comparison_operators": {
        "match": {
          "argument_type": {
            "name": "integer",
            "type": "named"
          },
          "type": "custom"
        },
        "match_phrase": {
          "argument_type": {
            "name": "integer",
            "type": "named"
          },
          "type": "custom"
        },
        "term": {
          "type": "equal"
        },
        "terms": {
          "argument_type": {
            "element_type": {
              "name": "integer",
              "type": "named"
            },
            "type": "array"
          },
          "type": "custom"
        }
      },
      "representation": {
        "type": "integer"
      }
    },
    "keyword": {
      "aggregate_functions": {
        "cardinality": {
          "result_type": {
            "name": "integer",
            "type": "named"
          }
        },
        "string_stats": {
          "result_type": {
            "name": "string_stats",
            "type": "named"
          }
        },
        "value_count": {
          "result_type": {
            "name": "integer",
            "type": "named"
          }
        }
      },
      "comparison_operators": {
        "match": {
          "argument_type": {
            "name": "keyword",
            "type": "named"
          },
          "type": "custom"
        },
        "match_bool_prefix": {
          "argument_type": {
            "name": "keyword",
            "type": "named"
          },
          "type": "custom"
        },
        "match_phrase": {
          "argument_type": {
            "name": "keyword",
            "type": "named"
          },
          "type": "custom"
        },
        "prefix": {
          "argument_type": {
            "name": "keyword",
            "type": "named"
          },
          "type": "custom"
        },
        "regexp": {
          "argument_type": {
            "name": "keyword",
            "type": "named"
          },
          "type": "custom"
        },
        "term": {
          "type": "equal"
        },
        "terms": {
          "argument_type": {
            "element_type": {
              "name": "keyword",
              "type": "named"
            },
            "type": "array"
          },
          "type": "custom"
        },
        "wildcard": {
          "argument_type": {
            "name": "keyword",
            "type": "named"
          },
          "type": "custom"
        }
      },
      "representation": {
        "type": "string"
      }
    },
    "long": {
      "aggregate_functions": {
        "avg": {
          "result_type": {
            "name": "long",
            "type": "named"
          }
        },
        "cardinality": {
          "result_type": {
            "name": "integer",
            "type": "named"
          }
        },
        "max": {
          "result_type": {
            "name": "long",
            "type": "named"
          }
        },
        "min": {
          "result_type": {
            "name": "long",
            "type": "named"
          }
        },
        "stats": {
          "result_type": {
            "name": "stats",
            "type": "named"
          }
        },
        "sum": {
          "result_type": {
            "name": "long",
            "type": "named"
          }
        },
        "value_count": {
          "result_type": {
            "name": "integer",
            "type": "named"
          }
        }
      },
      "comparison_operators": {
        "match": {
          "argument_type": {
            "name": "long",
            "type": "named"
          },
          "type": "custom"
        },
        "match_phrase": {
          "argument_type": {
            "name": "long",
            "type": "named"
          },
          "type": "custom"
        },
        "term": {
          "type": "equal"
        },
        "terms": {
          "argument_type": {
            "element_type": {
              "name": "long",
              "type": "named"
            },
            "type": "array"
          },
          "type": "custom"
        }
      },
      "representation": {
        "type": "integer"
      }
    },
    "text": {
      "aggregate_functions": {},
      "comparison_operators": {
        "match": {
          "argument_type": {
            "name": "text",
            "type": "named"
          },
          "type": "custom"
        },
        "match_bool_prefix": {
          "argument_type": {
            "name": "text",
            "type": "named"
          },
          "type": "custom"
        },
        "match_phrase": {
          "argument_type": {
            "name": "text",
            "type": "named"
          },
          "type": "custom"
        },
        "match_phrase_prefix": {
          "argument_type": {
            "name": "text",
            "type": "named"
          },
          "type": "custom"
        },
        "prefix": {
          "argument_type": {
            "name": "text",
            "type": "named"
          },
          "type": "custom"
        },
        "regexp": {
          "argument_type": {
            "name": "text",
            "type": "named"
          },
          "type": "custom"
        },
        "term": {
          "type": "equal"
        },
        "terms": {
          "argument_type": {
            "element_type": {
              "name": "text",
              "type": "named"
            },
            "type": "array"
          },
          "type": "custom"
        },
        "wildcard": {
          "argument_type": {
            "name": "text",
            "type": "named"
          },
          "type": "custom"
        }
      },
      "representation": {
        "type": "string"
      }
    },
    "text.keyword": {
      "aggregate_functions": {
        "cardinality": {
          "result_type": {
            "name": "integer",
            "type": "named"
          }
        },
        "string_stats": {
          "result_type": {
            "name": "string_stats",
            "type": "named"
          }
        },
        "value_count": {
          "result_type": {
            "name": "integer",
            "type": "named"
          }
        }
      },
      "comparison_operators": {
        "match": {
          "argument_type": {
            "name": "keyword",
            "type": "named"
          },
          "type": "custom"
        },
        "match_bool_prefix": {
          "argument_type": {
            "name": "keyword",
            "type": "named"
          },
          "type": "custom"
        },
        "match_phrase": {
          "argument_type": {
            "name": "keyword",
            "type": "named"
          },
          "type": "custom"
        },
        "prefix": {
          "argument_type": {
            "name": "keyword",
            "type": "named"
          },
          "type": "custom"
        },
        "regexp": {
          "argument_type": {
            "name": "keyword",
            "type": "named"
          },
          "type": "custom"
        },
        "term": {
          "type": "equal"
        },
        "terms": {
          "argument_type": {
            "element_type": {
              "name": "keyword",
              "type": "named"
            },
            "type": "array"
          },
          "type": "custom"
        },
        "wildcard": {
          "argument_type": {
            "name": "keyword",
            "type": "named"
          },
          "type": "custom"
        }
      },
      "representation": {
        "type": "string"
      }
    }
  }
}`

var tests = []struct {
	name string
	config   string
	schema  string
}{
	{
		name: "Identification",
		config: CONFIGURATION_IDENTIFICATION,
		schema: SCHEMA_IDENTIFICATION,
	},
	{
		name: "Books",
		config: CONFIGURATION_BOOKS,
		schema: SCHEMA_BOOKS,
	},
} 


func initTest(t *testing.T, configuration string) (state *types.State, config types.Configuration) {
	state = &types.State{
		TelemetryState:           nil,
		Client:                   nil,
		SupportedSortFields:      make(map[string]interface{}),
		SupportedAggregateFields: make(map[string]interface{}),
		SupportedFilterFields:    make(map[string]interface{}),
		NestedFields:             make(map[string]interface{}),
		ElasticsearchInfo:        nil,
	}

	// Unmarshal the JSON string into the Configuration struct
	err := json.Unmarshal([]byte(configuration), &config)
	if err != nil {
		t.Fatalf("Error unmarshalling JSON: %v", err)
	}

	return state, config
}

func TestSchema(t *testing.T) {
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state, config := initTest(t, tt.config)

			schema := connector.ParseConfigurationToSchema(&config, state)

			jsonData, err := json.Marshal(schema)
			if err != nil {
				t.Fatalf("Error marshalling schema: %v", err)
			}

			// fmt.Printf("\n\n\nSchema: %s\n\n", string(jsonData));

			assert.JSONEq(t, tt.schema, string(jsonData), "Schema does not match");
		})
	}
}
