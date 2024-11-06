package internal

// Given a fieldMap, this function extracts the type and all subtypes (if present)
func ExtractTypes(fieldMap map[string]interface{}) (fieldAndSubfields []string) {
	if subFields, ok := HasSubfields(fieldMap); ok {
		for _, subFieldData := range subFields {
			fieldAndSubfields = append(fieldAndSubfields, ExtractTypes(subFieldData.(map[string]interface{}))...)
		}
	}

	SortTypesByPriority(fieldAndSubfields)

	fieldType, _ := FieldTypeIsScalar(fieldMap)
	fieldAndSubfields = append([]string{fieldType}, fieldAndSubfields...)
	return fieldAndSubfields
}

func HasSubfields(fieldMap map[string]interface{}) (subFields map[string]interface{}, ok bool) {
	subFields, ok = fieldMap["fields"].(map[string]interface{})
	return subFields, ok
}

/**
 * This function checks if a field is a scalar field
 * Scalar field here refers to a field that does not have any nested fields/properties
 */
 func FieldTypeIsScalar(fieldMap map[string]interface{}) (fieldType string, isFieldScalar bool) {
	fieldType, ok := fieldMap["type"].(string)
	return fieldType, ok && fieldType != "nested" && fieldType != "object" && fieldType != "flattened"
}

// IsSortSupported checks if a field type is supported for sorting
// based on fielddata and unsupported sort data types.
func IsSortSupported(fieldType string, fieldDataEnalbed bool) bool {
	if fieldDataEnalbed {
		return true
	}
	for _, unSupportedType := range UnSupportedSortDataTypes {
		if fieldType == unSupportedType {
			return false
		}
	}
	return true
}

// IsAggregateSupported checks if a field type is supported for aggregation
// based on fielddata and unsupported aggregate data types.
func IsAggregateSupported(fieldType string, fieldDataEnalbed bool) bool {
	if fieldDataEnalbed {
		return true
	}

	for _, unSupportedType := range UnSupportedAggregateTypes {
		if fieldType == unSupportedType {
			return false
		}
	}

	return true
}

func IsFieldDtaEnabled(fieldMap map[string]interface{}) bool {
	fieldDataEnalbed := false
	if fieldData, ok := fieldMap["fielddata"].(bool); ok {
		fieldDataEnalbed = fieldData
	}
	return fieldDataEnalbed
}