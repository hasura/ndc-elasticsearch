package internal

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

var fieldsAndSubfieldsTests = []struct {
	name                        string
	fieldMapStr                 string
	wantLegacyFieldAndSubfields []string
	wantType                    string
	wantSubFieldsMap            map[string]string
}{
	{
		name: "nested_field",
		fieldMapStr: `{
        	"type": "text",
        	"fields": {
          		"raw": { 
            		"type":  "keyword"
          		}
        	}
      	}`,
		wantLegacyFieldAndSubfields: []string{"text", "keyword"},
		wantType:                    "text",
		wantSubFieldsMap:            map[string]string{"keyword": "raw"},
	},
	{
		name: "nested_fields",
		fieldMapStr: `{
        	"type": "text",
        	"fields": {
          		"raw": {
            		"type":  "keyword"
          		},
				"raw_int": {
            		"type":  "integer"
          		},
				"raw_double": { 
            		"type":  "double"
          		},
				"simple_boolean": {
            		"type":  "boolean"
          		},
				"ip": {
            		"type":  "ip"
          		},
				"version": {
            		"type":  "version"
          		}
        	}
      	}`,
		wantLegacyFieldAndSubfields: []string{
			"text",
			"boolean",
			"integer",
			"double",
			"ip",
			"keyword",
			"version",
		},
		wantType: "text",
		wantSubFieldsMap: map[string]string{
			"keyword": "raw",
			"integer": "raw_int",
			"double":  "raw_double",
			"boolean": "simple_boolean",
			"ip":      "ip",
			"version": "version",
		},
	},
	{
		name: "no_nested_field",
		fieldMapStr: `{
        	"type": "keyword"
      	}`,
		wantLegacyFieldAndSubfields: []string{"keyword"},
		wantType:                    "keyword",
		wantSubFieldsMap:            map[string]string{},
	},
	{
		name: "nested_field_type_same_as_field_type",
		fieldMapStr: `{ 
        	"type": "text",
        	"fields": {
          		"english": { 
            		"type": "text",
            		"analyzer": "english"
          		}
        	}
      	}`,
		wantLegacyFieldAndSubfields: []string{"text"},
		wantType: "text",
		wantSubFieldsMap: map[string]string{},
	},
	{
		name: "duplicate_nested_field",
		fieldMapStr: `{
        	"type": "text",
        	"fields": {
          		"raw": { 
            		"type":  "keyword"
          		},
				"raw": { 
            		"type":  "keyword"
          		}
        	}
      	}`,
		wantLegacyFieldAndSubfields: []string{"text", "keyword"},
		wantType:                    "text",
		wantSubFieldsMap:            map[string]string{"keyword": "raw"},
	},
}

func TestExtractTypes(t *testing.T) {
	for _, tt := range fieldsAndSubfieldsTests {
		t.Run(tt.name, func(t *testing.T) {
			var fieldMap map[string]interface{}
			err := json.Unmarshal([]byte(tt.fieldMapStr), &fieldMap)
			assert.NoError(t, err, "Error unmarshalling JSON")

			fieldAndSubfields, fieldType, subFieldsMap := ExtractTypes(fieldMap)

			assert.Equal(t, tt.wantLegacyFieldAndSubfields, fieldAndSubfields)
			assert.Equal(t, tt.wantType, fieldType)
			assert.Equal(t, tt.wantSubFieldsMap, subFieldsMap)
		})
	}
}
