package ginai

import (
	"fmt"
	"math"
	"slices"
)

// ValidateArguments checks an LLM-produced argument object against a tool schema.
func ValidateArguments(schema *JSONSchema, args map[string]any) error {
	if schema == nil {
		return nil
	}
	if schema.Type != "" && schema.Type != "object" {
		return fmt.Errorf("root schema must be object")
	}
	if args == nil {
		args = map[string]any{}
	}
	for _, name := range schema.Required {
		if isMissing(args[name]) {
			return fmt.Errorf("missing required argument %q", name)
		}
	}
	for name, value := range args {
		fieldSchema, ok := schema.Properties[name]
		if !ok {
			continue
		}
		if isMissing(value) {
			continue
		}
		if err := validateValue(name, fieldSchema, value); err != nil {
			return err
		}
	}
	return nil
}

func validateValue(name string, schema *JSONSchema, value any) error {
	switch schema.Type {
	case "string":
		text, ok := value.(string)
		if !ok {
			return fmt.Errorf("argument %q must be string", name)
		}
		if len(schema.Enum) > 0 && !slices.Contains(schema.Enum, text) {
			return fmt.Errorf("argument %q must be one of %v", name, schema.Enum)
		}
	case "integer":
		if !isInteger(value) {
			return fmt.Errorf("argument %q must be integer", name)
		}
	case "number":
		if !isNumber(value) {
			return fmt.Errorf("argument %q must be number", name)
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("argument %q must be boolean", name)
		}
	case "array":
		items, ok := value.([]any)
		if !ok {
			return fmt.Errorf("argument %q must be array", name)
		}
		for i, item := range items {
			if err := validateValue(fmt.Sprintf("%s[%d]", name, i), schema.Items, item); err != nil {
				return err
			}
		}
	case "object":
		object, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("argument %q must be object", name)
		}
		if err := ValidateArguments(schema, object); err != nil {
			return fmt.Errorf("argument %q: %w", name, err)
		}
	}
	return nil
}

func isMissing(value any) bool {
	switch typed := value.(type) {
	case nil:
		return true
	case string:
		return typed == ""
	default:
		return false
	}
}

func isInteger(value any) bool {
	switch typed := value.(type) {
	case int, int8, int16, int32, int64:
		return true
	case uint, uint8, uint16, uint32, uint64:
		return true
	case float64:
		return math.Trunc(typed) == typed
	default:
		return false
	}
}

func isNumber(value any) bool {
	switch value.(type) {
	case int, int8, int16, int32, int64:
		return true
	case uint, uint8, uint16, uint32, uint64:
		return true
	case float32, float64:
		return true
	default:
		return false
	}
}
