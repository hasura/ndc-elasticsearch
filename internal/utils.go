package internal

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"
)

const (
	letterIdxBits = 6                    // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
	alphaDigits   = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
)

var src = rand.NewSource(time.Now().UnixNano())

// GenRandomString generate random string with fixed length
func GenRandomString(n int) string {
	allowedChars := alphaDigits
	sb := strings.Builder{}
	sb.Grow(n)
	// A src.Int63() generates 63 random bits, enough for letterIdxMax characters!
	for i, cache, remain := n-1, src.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = src.Int63(), letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < len(allowedChars) {
			sb.WriteByte(allowedChars[idx])
			i--
		}
		cache >>= letterIdxBits
		remain--
	}

	return sb.String()
}

// DeepEqual checks if both values are recursively equal
// used for testing purpose only
func DeepEqual(v1, v2 any) bool {
	if reflect.DeepEqual(v1, v2) {
		return true
	}

	bytesA, _ := json.Marshal(v1)
	bytesB, _ := json.Marshal(v2)
	if string(bytesA) == string(bytesB) {
		return true
	}

	switch reflect.ValueOf(v1).Kind() {
	case reflect.Slice, reflect.Array:
		var values1 []map[string]any
		var values2 []map[string]any
		if err := json.Unmarshal(bytesA, &values1); err == nil {
			if err2 := json.Unmarshal(bytesB, &values2); err2 != nil {
				return false
			}
			if len(values1) != len(values2) {
				return false
			}

			for i, value1 := range values1 {
				if !DeepEqual(value1, values2[i]) {
					j1, _ := json.Marshal(value1)
					j2, _ := json.Marshal(values2[i])
					log.Printf("deep equality is failed at index: %d\n value 1: %s\n value 2: %s\n", i, string(j1), string(j2))
					return false
				}
			}
			return true
		}
	case reflect.Struct, reflect.Map:
		var map1 map[string]any
		var map2 map[string]any
		if err := json.Unmarshal(bytesA, &map1); err == nil {
			if err2 := json.Unmarshal(bytesB, &map2); err2 != nil {
				return false
			}
			if len(map1) != len(map2) {
				return false
			}
			for k, v1 := range map1 {
				v2, ok := map2[k]
				if !ok || !DeepEqual(v1, v2) {
					j1, _ := json.Marshal(v1)
					j2, _ := json.Marshal(v2)
					log.Printf("deep equality is failed at key: %s\n expected	: %s\n got			: %s\n", k, string(j1), string(j2))
					return false
				}
			}
			return true
		}
	}

	var x1 any
	var x2 any
	_ = json.Unmarshal(bytesA, &x1)
	_ = json.Unmarshal(bytesB, &x2)
	return reflect.DeepEqual(x1, x2)
}

func ValidateAggregateOperation(supportedFields map[string]interface{}, collection, field string) string {
	return validateOperation(supportedFields, collection, field)
}

func ValidateSortOperation(supportedFields map[string]interface{}, collection, field string) string {
	return validateOperation(supportedFields, collection, field)
}

func validateOperation(supportedFields map[string]interface{}, collection, field string) string {
	supportedFieldsMap, ok := supportedFields[collection].(map[string]string)
	if !ok {
		return field
	}

	validField, ok := supportedFieldsMap[field]
	if !ok {
		return ""
	}

	return validField
}

// ReadJsonFileUsingDecoder reads a JSON file using a decoder.
func ReadJsonFileUsingDecoder(filename string) (map[string]interface{}, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %w", filename, err)
	}
	defer file.Close()

	var data map[string]interface{}
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&data); err != nil {
		return nil, fmt.Errorf("failed to decode JSON for file %s: %w", filename, err)
	}

	return data, nil
}

// WriteJsonFile writes the given byte slice to a temporary file first
// and then renaming it to the destination to avoid partial writes in case of a failure.
func WriteJsonFile(filename string, data []byte) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	_, err = file.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write to file: %w", err)
	}

	return nil
}

// FileExists checks whether a file exists and returns a boolean value.
func FileExists(filename string) bool {
	_, err := os.Stat(filename)
	return !os.IsNotExist(err)
}

// contains checks if a string slice contains a specific element.
func Contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

// Use this function to access files that are provided by the user (like certificates, etc).
//
// This function is responsible for generating the correct path and accessing the file.
// The reason for generating a path that's different from the one that the user provided
// is because there are two components that will be accessing the file:
//
// 1. The CLI Pluign: This will access the file from the ddn directory on the user's file system
//
// 2. The Docker Container: This will access the file from the /etc/connector/ directory
//
// The path of the file will change based on the component that is accessing it
func GetUsersFile(filePath string) ([]byte, error) {
	if os.Getenv("HASURA_PLUGIN_CONNECTOR_CONTEXT_PATH") != "" {
		// the plugin is accessing the file
		// we'll prepend the value of this env var to the file path
		filePath = filepath.Join(os.Getenv("HASURA_PLUGIN_CONNECTOR_CONTEXT_PATH"), filePath)
	} else if os.Getenv("HASURA_CONFIGURATION_DIRECTORY") != "" {
		// the docker container is accessing the file
		filePath = filepath.Join(os.Getenv("HASURA_CONFIGURATION_DIRECTORY"), filePath)
	} else {
		// since no env var is set, we'll assume that the docker container is accessing the file
		// and the file is mounted to the /etc/connector/ directory, according to the spec:
		// https://github.com/hasura/ndc-hub/blob/a764938e13fa3cc719745edaff73b584a936e3ef/rfcs/0000-deployment.md?plain=1#L23
		filePath = filepath.Join("/etc/connector/", filePath)
	}

	if !FileExists(filePath) {
		return nil, fmt.Errorf("file not found at %s", filePath)
	}

	fileContent, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	return fileContent, nil
}
