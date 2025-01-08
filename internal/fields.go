package internal

// Given a fieldMap, this function extracts the type and all subtypes (if present)
//
// **RETURNS**
// 1. legacyFieldAndSubfields: a slice of strings containing the field type as the first element and all subfields in the following elements. Called legacy because it supports older code that expects this format. Please refrain from using this in newer functions.
// 2. fieldType: the type of the field
// 3. subFieldsMap: a map of types to their subfields
func ExtractTypes(fieldMap map[string]interface{}) (legacyFieldAndSubfields []string, fieldType string, subFieldsMap map[string]string) {
	subFieldsMap = make(map[string]string) // subFieldsMap is a map of types and their subFields
	fieldType, _ = FieldTypeIsScalar(fieldMap)

	if subFields, ok := HasSubfields(fieldMap); ok {
		for subField, subFieldData := range subFields {
			subFieldType, ok := subFieldData.(map[string]interface{})["type"].(string)
			if !ok {
				continue
			}
			if subFieldType == fieldType {
				// since the subfield type is the same as the field type, we will consider the main field type
				continue
			}
			if _, ok := subFieldsMap[subFieldType]; ok {
				// subFieldType already exists, skip
				continue
			}
			legacyFieldAndSubfields = append(legacyFieldAndSubfields, subFieldType)
			subFieldsMap[subFieldType] = subField
		}
	}

	SortTypesByPriority(legacyFieldAndSubfields)

	legacyFieldAndSubfields = append([]string{fieldType}, legacyFieldAndSubfields...)
	return legacyFieldAndSubfields, fieldType, subFieldsMap
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
