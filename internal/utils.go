package internal

import (
	"encoding/json"
	"log"
	"math/rand"
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
