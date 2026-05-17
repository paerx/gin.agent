package ginai

import "testing"

type sampleSchemaReq struct {
	Wallet string `json:"wallet" ai:"desc=用户钱包地址,required"`
	Field  string `json:"field" ai:"desc=要更新的字段,enum=nickname|avatar|bio,required"`
	Value  string `json:"value" ai:"desc=新的字段值"`
}

func TestSchemaFromStruct(t *testing.T) {
	schema, err := SchemaFromStruct(sampleSchemaReq{})
	if err != nil {
		t.Fatalf("SchemaFromStruct() error = %v", err)
	}
	if schema.Type != "object" {
		t.Fatalf("schema type = %s", schema.Type)
	}
	if got := schema.Properties["wallet"].Description; got != "用户钱包地址" {
		t.Fatalf("wallet description = %s", got)
	}
	if got := len(schema.Properties["field"].Enum); got != 3 {
		t.Fatalf("field enum count = %d", got)
	}
	if len(schema.Required) != 2 {
		t.Fatalf("required count = %d", len(schema.Required))
	}
}

func TestSchemaFromStructUnsupportedType(t *testing.T) {
	type bad struct {
		When complex128 `json:"when"`
	}
	if _, err := SchemaFromStruct(bad{}); err == nil {
		t.Fatal("expected unsupported type error")
	}
}
