package ginai

import "testing"

func TestValidateArguments(t *testing.T) {
	schema, err := SchemaFromStruct(sampleSchemaReq{})
	if err != nil {
		t.Fatalf("SchemaFromStruct() error = %v", err)
	}
	if err := ValidateArguments(schema, map[string]any{
		"wallet": "0xabc",
		"field":  "nickname",
		"value":  "Paer",
	}); err != nil {
		t.Fatalf("ValidateArguments() error = %v", err)
	}
}

func TestValidateArgumentsRequired(t *testing.T) {
	schema, err := SchemaFromStruct(sampleSchemaReq{})
	if err != nil {
		t.Fatalf("SchemaFromStruct() error = %v", err)
	}
	if err := ValidateArguments(schema, map[string]any{"field": "nickname"}); err == nil {
		t.Fatal("expected missing required argument error")
	}
}

func TestValidateArgumentsEnum(t *testing.T) {
	schema, err := SchemaFromStruct(sampleSchemaReq{})
	if err != nil {
		t.Fatalf("SchemaFromStruct() error = %v", err)
	}
	if err := ValidateArguments(schema, map[string]any{
		"wallet": "0xabc",
		"field":  "role",
	}); err == nil {
		t.Fatal("expected enum validation error")
	}
}
