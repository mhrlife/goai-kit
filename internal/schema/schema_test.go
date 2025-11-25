package schema

import (
	"reflect"
	"testing"
)

func TestMarshalToSchema(t *testing.T) {
	type x struct {
		A string `json:"a"`
		B string `json:"b" jsonschema:"description=hello"`
	}

	marshalled := MarshalToSchema(x{})

	expected := map[string]any{
		"$id":                  "https://github.com/mhrlife/goai-kit/x",
		"additionalProperties": false,
		"properties": map[string]any{
			"a": map[string]any{
				"type": "string",
			},
			"b": map[string]any{
				"description": "hello",
				"type":        "string",
			},
		},
		"required": []any{"a", "b"},
		"type":     "object",
	}

	if !reflect.DeepEqual(marshalled, expected) {
		t.Errorf("MarshalToSchema() = %v, want %v", marshalled, expected)
	}
}
