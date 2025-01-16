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

// GetBestFieldOrSubFieldForAggregation returns the best field or subfield for the given aggregation operator
func GetBestFieldOrSubFieldForAggregation(fieldPath, fieldType string, subFieldMap map[string]string, operator string) (bestFieldOrSubField string, operatorFound bool) {
	// call the getBestFieldOrSubFieldForOperators function with the aggregation operators
	return getBestFieldOrSubFieldForOperators(fieldPath, fieldType, subFieldMap, operator, NumericalAggregations, TermLevelAggregations, FullTextAggregations)
}

// GetBestFieldOrSubFieldForQuery returns the best field or subfield for the given query operator
func GetBestFieldOrSubFieldForQuery(fieldPath, fieldType string, subFieldMap map[string]string, operator string) (bestFieldOrSubField string, operatorFound bool) {
	// call the getBestFieldOrSubFieldForOperators function with the query operators
	return getBestFieldOrSubFieldForOperators(fieldPath, fieldType, subFieldMap, operator, NumericalQueries, TermLevelQueries, FullTextQueries)
}

// getBestFieldOrSubFieldForOperators returns the best field or subfield for the given operator
func getBestFieldOrSubFieldForOperators(fieldPath, fieldType string, subFieldMap map[string]string, operator string, numericOperators, termLevelOperators, fullTextOperators map[string]bool) (bestFieldOrSubField string, operatorFound bool) {
	bestFieldOrSubFieldFound := false

	if numericOperators[operator] {
		// if the operator is a numeric operator, get the best field or subfield from the numeric family
		bestFieldOrSubField, bestFieldOrSubFieldFound = GetBestFieldOrSubFieldForFamily(fieldPath, fieldType, subFieldMap, NumericFamilyOfTypes)
		operatorFound = true
	}
	if termLevelOperators[operator] && !bestFieldOrSubFieldFound {
		// if the operator is a term level operator, get the best field or subfield from the keyword family
		bestFieldOrSubField, bestFieldOrSubFieldFound = GetBestFieldOrSubFieldForFamily(fieldPath, fieldType, subFieldMap, KeywordFamilyOfTypes)
		operatorFound = true
	}
	if fullTextOperators[operator] && !bestFieldOrSubFieldFound {
		// if the operator is a full text operator, get the best field or subfield from the text family
		bestFieldOrSubField, _ = GetBestFieldOrSubFieldForFamily(fieldPath, fieldType, subFieldMap, TextFamilyOfTypes)
		operatorFound = true
	}
	return bestFieldOrSubField, operatorFound
}

// GetBestFieldOrSubFieldForFamily returns the best field or the `field.subtype` match found in bestTypesFamily
func GetBestFieldOrSubFieldForFamily(fieldPath, fieldType string, fieldSubTypes map[string]string, bestTypesFamily map[string]bool) (bestField string, typeFound bool) {
	if bestTypesFamily[fieldType] {
		// if the field type is in the best types family, return the field path
		return fieldPath, true
	} else if len(fieldSubTypes) == 0 {
		// if the field has no subtypes, return the field path
		return fieldPath, false
	} else if len(fieldSubTypes) > 0 {
		// if the field has subtypes, return the first matching subfield appended to field path
		for subType, subField := range fieldSubTypes {
			if bestTypesFamily[subType] {
				return fieldPath + "." + subField, true
			}
		}
	}
	// nothing found, return the field path
	return fieldPath, false
}
