package connector

import (
	"encoding/json"
	"testing"

	"github.com/hasura/ndc-elasticsearch/types"
	"github.com/stretchr/testify/assert"
)

func TestExtractDocumentNestedObjects(t *testing.T) {
	selected := map[string]types.Field{
		"caseId": {Name: "caseId"},
		"officers": {Name: "officers", Fields: map[string]types.Field{
			"employeeId": {Name: "employeeId"},
			"label":      {Name: "label"},
		}},
	}

	for _, tc := range []struct {
		name   string
		source string
		want   string
	}{
		{"missing field", `{"caseId": "c"}`, `{"caseId": "c", "officers": []}`},
		{"null field", `{"caseId": "c", "officers": null}`, `{"caseId": "c", "officers": []}`},
		{"empty array", `{"caseId": "c", "officers": []}`, `{"caseId": "c", "officers": []}`},
		{"single object", `{"caseId": "c", "officers": {"employeeId": "id-a", "label": "a"}}`, `{"caseId": "c", "officers": [{"employeeId": "id-a", "label": "a"}]}`},
		{"object without key", `{"caseId": "c", "officers": [{"label": "no-id"}]}`, `{"caseId": "c", "officers": [{"employeeId": null, "label": "no-id"}]}`},
		{"array with null element", `{"caseId": "c", "officers": [{"employeeId": "id-a", "label": "a"}, null]}`, `{"caseId": "c", "officers": [{"employeeId": "id-a", "label": "a"}]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var source map[string]interface{}
			assert.NoError(t, json.Unmarshal([]byte(tc.source), &source))
			got, err := json.Marshal(extractDocument(source, selected))
			assert.NoError(t, err)
			assert.JSONEq(t, tc.want, string(got))
		})
	}
}

func TestExtractDocumentDeeplyNestedMissingObject(t *testing.T) {
	selected := map[string]types.Field{
		"policy": {Name: "policy", Fields: map[string]types.Field{
			"holder": {Name: "holder", Fields: map[string]types.Field{
				"name": {Name: "name"},
			}},
		}},
	}

	var source map[string]interface{}
	assert.NoError(t, json.Unmarshal([]byte(`{"policy": {}}`), &source))
	got, err := json.Marshal(extractDocument(source, selected))
	assert.NoError(t, err)
	assert.JSONEq(t, `{"policy": [{"holder": []}]}`, string(got))
}
