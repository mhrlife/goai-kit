package schema

import (
	"encoding/json"
	"log"
	"reflect"

	"github.com/invopop/jsonschema"
)

func InferJSONSchema(x any) (s *jsonschema.Schema) {
	r := jsonschema.Reflector{
		DoNotReference: true,
		Mapper: func(t reflect.Type) *jsonschema.Schema {
			if t.Kind() == reflect.Slice && t.Elem().Kind() == reflect.Interface {
				return &jsonschema.Schema{
					Type: "array",
					Items: &jsonschema.Schema{
						AdditionalProperties: jsonschema.TrueSchema,
					},
				}
			}
			return nil
		},
	}
	s = r.Reflect(x)
	s.Version = ""
	return s
}

func asMap(s *jsonschema.Schema) map[string]any {
	jsb, err := s.MarshalJSON()
	if err != nil {
		log.Panicf("failed to marshal schema: %v", err)
	}

	// Check if the marshaled JSON is "true" (indicates an empty schema)
	if string(jsb) == "true" {
		return make(map[string]any)
	}

	var m map[string]any
	err = json.Unmarshal(jsb, &m)
	if err != nil {
		log.Panicf("failed to unmarshal schema: %v", err)
	}
	return m
}

func MarshalToSchema(x any) map[string]any {
	s := InferJSONSchema(x)
	m := asMap(s)
	return m
}
