package ginai

import "testing"

func TestRegistryRegister(t *testing.T) {
	registry := NewRegistry()
	tool := &Tool{
		Name:        "get_user",
		Description: "query user",
		Method:      "GET",
		Path:        "/users",
		Params: struct {
			Wallet string `json:"wallet"`
		}{},
		ReadOnly: true,
	}
	if err := registry.Register(tool); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if _, ok := registry.Get(tool.Name); !ok {
		t.Fatal("tool not found after register")
	}
	if err := registry.Register(tool); err == nil {
		t.Fatal("expected duplicate tool error")
	}
}

func TestRegistryWriteDefaultsToNeedConfirm(t *testing.T) {
	registry := NewRegistry()
	tool := &Tool{
		Name:        "update_user",
		Description: "update user",
		Method:      "POST",
		Path:        "/users",
		Params:      struct{}{},
	}
	if err := registry.Register(tool); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	stored, _ := registry.Get(tool.Name)
	if !stored.NeedConfirm {
		t.Fatal("expected NeedConfirm to default true for write tools")
	}
}
