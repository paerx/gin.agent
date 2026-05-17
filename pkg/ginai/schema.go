package ginai

import (
	"fmt"
	"reflect"
	"strings"
)

// JSONSchema is a compact v0.1 schema format that works with tool calling.
type JSONSchema struct {
	Type        string                 `json:"type,omitempty"`
	Description string                 `json:"description,omitempty"`
	Enum        []string               `json:"enum,omitempty"`
	Properties  map[string]*JSONSchema `json:"properties,omitempty"`
	Items       *JSONSchema            `json:"items,omitempty"`
	Required    []string               `json:"required,omitempty"`
}

type aiTag struct {
	Description string
	Required    bool
	Enum        []string
}

// SchemaFromStruct converts a request struct into a JSON schema.
func SchemaFromStruct(params any) (*JSONSchema, error) {
	if params == nil {
		return &JSONSchema{Type: "object", Properties: map[string]*JSONSchema{}}, nil
	}

	rt := reflect.TypeOf(params)
	for rt.Kind() == reflect.Pointer {
		rt = rt.Elem()
	}
	if rt.Kind() != reflect.Struct {
		return nil, fmt.Errorf("params must be a struct, got %s", rt.Kind())
	}

	schema := &JSONSchema{
		Type:       "object",
		Properties: make(map[string]*JSONSchema),
	}

	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		if field.PkgPath != "" {
			continue
		}

		name := jsonFieldName(field)
		if name == "" {
			continue
		}

		fieldSchema, required, err := schemaForField(field)
		if err != nil {
			return nil, fmt.Errorf("field %s: %w", field.Name, err)
		}
		schema.Properties[name] = fieldSchema
		if required {
			schema.Required = append(schema.Required, name)
		}
	}

	return schema, nil
}

func schemaForField(field reflect.StructField) (*JSONSchema, bool, error) {
	tag := parseAITag(field.Tag.Get("ai"))
	base, err := schemaForType(field.Type)
	if err != nil {
		return nil, false, err
	}
	if tag.Description != "" {
		base.Description = tag.Description
	}
	if len(tag.Enum) > 0 {
		if base.Type != "string" {
			return nil, false, fmt.Errorf("enum is only supported on string fields")
		}
		base.Enum = tag.Enum
	}
	return base, tag.Required, nil
}

func schemaForType(rt reflect.Type) (*JSONSchema, error) {
	for rt.Kind() == reflect.Pointer {
		rt = rt.Elem()
	}

	switch rt.Kind() {
	case reflect.String:
		return &JSONSchema{Type: "string"}, nil
	case reflect.Int, reflect.Int64:
		return &JSONSchema{Type: "integer"}, nil
	case reflect.Float64:
		return &JSONSchema{Type: "number"}, nil
	case reflect.Bool:
		return &JSONSchema{Type: "boolean"}, nil
	case reflect.Struct:
		return SchemaFromStruct(reflect.New(rt).Elem().Interface())
	case reflect.Slice:
		itemType := rt.Elem()
		itemSchema, err := schemaForType(itemType)
		if err != nil {
			return nil, err
		}
		if itemSchema.Type == "object" {
			return nil, fmt.Errorf("nested struct slices are not supported in v0.1")
		}
		return &JSONSchema{
			Type:  "array",
			Items: itemSchema,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported type %s", rt.Kind())
	}
}

func jsonFieldName(field reflect.StructField) string {
	tag := field.Tag.Get("json")
	if tag == "-" {
		return ""
	}
	if tag == "" {
		return field.Name
	}
	parts := strings.Split(tag, ",")
	if parts[0] == "" {
		return field.Name
	}
	return parts[0]
}

func parseAITag(raw string) aiTag {
	if raw == "" {
		return aiTag{}
	}
	var out aiTag
	parts := strings.Split(raw, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		switch {
		case part == "required":
			out.Required = true
		case strings.HasPrefix(part, "desc="):
			out.Description = strings.TrimPrefix(part, "desc=")
		case strings.HasPrefix(part, "enum="):
			value := strings.TrimPrefix(part, "enum=")
			if value == "" {
				continue
			}
			for _, item := range strings.Split(value, "|") {
				item = strings.TrimSpace(item)
				if item != "" {
					out.Enum = append(out.Enum, item)
				}
			}
			if len(out.Enum) == 0 {
				for _, item := range strings.Split(value, ";") {
					item = strings.TrimSpace(item)
					if item != "" {
						out.Enum = append(out.Enum, item)
					}
				}
			}
			if len(out.Enum) == 0 {
				for _, item := range strings.Split(value, ":") {
					item = strings.TrimSpace(item)
					if item != "" {
						out.Enum = append(out.Enum, item)
					}
				}
			}
			if len(out.Enum) == 0 {
				for _, item := range strings.Split(value, ",") {
					item = strings.TrimSpace(item)
					if item != "" {
						out.Enum = append(out.Enum, item)
					}
				}
			}
		}
	}
	return out
}
