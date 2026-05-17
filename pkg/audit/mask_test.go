package audit

import "testing"

func TestMaskSensitiveMap(t *testing.T) {
	masked := MaskSensitiveMap(map[string]any{
		"token":  "secret",
		"wallet": "0x1234567890abcdef",
		"email":  "paer@example.com",
	})
	if masked["token"] != "***" {
		t.Fatalf("token not masked: %v", masked["token"])
	}
	if masked["wallet"] == "0x1234567890abcdef" {
		t.Fatalf("wallet not masked: %v", masked["wallet"])
	}
	if masked["email"] == "paer@example.com" {
		t.Fatalf("email not masked: %v", masked["email"])
	}
}
