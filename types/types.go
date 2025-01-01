package types

import (
	"fmt"
	"strings"

	"github.com/hasura/ndc-elasticsearch/elasticsearch"
	"github.com/hasura/ndc-elasticsearch/internal"
	"github.com/hasura/ndc-sdk-go/connector"
	"github.com/hasura/ndc-sdk-go/schema"
)

// State is the global state which is shared for every connector request.
type State struct {
	*connector.TelemetryState
	Client                   *elasticsearch.Client
	SupportedSortFields      map[string]interface{}
	SupportedAggregateFields map[string]interface{}
	SupportedFilterFields    map[string]interface{}
	ElasticsearchInfo        map[string]interface{}
	Schema                   *schema.SchemaResponse
	NestedFields             map[string]interface{}
	Configuration            *Configuration
}

// Configuration contains required settings for the connector.
type Configuration struct {
	Indices map[string]interface{} `json:"indices"`
	Queries map[string]NativeQuery `json:"queries"`
}

func (c *Configuration) GetIndex(indexName string) (map[string]interface{}, error) {
	index, ok := c.Indices[indexName].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unable to find index: %s", indexName)
	}

	return index, nil
}

// returns the field object for the given field path from configuration
func (c *Configuration) GetFieldMap(indexName, fieldPath string) (map[string]interface{}, error) {
	index, err := c.GetIndex(indexName)
	if err != nil {
		return nil, err
	}

	mapping, ok := index["mappings"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unable to find mapping in index: %s", indexName)
	}

	splitFieldPath := strings.Split(fieldPath, ".")
	curFieldPath := ""

	fieldMap := make(map[string]interface{})

	for i, curFieldName := range splitFieldPath {
		if i == 0 {
			curFieldPath = curFieldName
		} else {
			curFieldPath = fmt.Sprintf("%s.%s", curFieldPath, curFieldName)
		}

		properties, ok := mapping["properties"].(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("unable to find properties in index `%s` for field `%s`", indexName, curFieldPath)
		}

		fieldMap, ok = properties[curFieldName].(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("unable to find field `%s` in index `%s` ", curFieldPath, indexName)
		}
		mapping = fieldMap
	}

	return fieldMap, nil

}

// GetFieldProperties returns the field type, subtypes and field data enabled for the given field path.
func (c *Configuration) GetFieldProperties(indexName, fieldPath string) (fieldType string, fieldSubTypes []string, fieldDataEnabled bool, err error) {
	fieldMap, err := c.GetFieldMap(indexName, fieldPath)
	if err != nil {
		return "", nil, false, err
	}

	fieldsAndSubfields := internal.ExtractTypes(fieldMap)

	fieldDataEnabled = internal.IsFieldDtaEnabled(fieldMap)

	if len(fieldsAndSubfields) == 1 {
		return fieldsAndSubfields[0], make([]string, 0), fieldDataEnabled, nil
	}

	return fieldsAndSubfields[0], fieldsAndSubfields[1:], fieldDataEnabled, nil
}

// NativeQuery contains the definition of the native query.
type NativeQuery struct {
	DSL        DSL                     `json:"dsl"`
	Index      string                  `json:"index"`
	ReturnType *ReturnType             `json:"return_type,omitempty"`
	Arguments  *map[string]interface{} `json:"arguments,omitempty"`
}

// DSL contains the dsl query of the native query.
type DSL struct {
	File     *string                 `json:"file,omitempty"`
	Internal *map[string]interface{} `json:"internal,omitempty"`
}

// ReturnType contains the return type of the native query.
type ReturnType struct {
	Kind     string                  `json:"kind"`
	Mappings *map[string]interface{} `json:"mappings,omitempty"`
}

// PostProcessor is used to post process the query response.
type PostProcessor struct {
	IsFields        bool
	StarAggregates  string
	ColumnAggregate map[string]bool
	IsIDSelected    bool
	SelectedFields  map[string]Field
}

// Field is used to represent a field in the query response.
type Field struct {
	Name   string
	Fields map[string]Field
}

type Variable string
