package transport

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"gin.agent/pkg/ginai"
)

func TestHTTPInvokerGET(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-GinAI-Internal-Token"); got != "token" {
			t.Fatalf("missing internal token: %s", got)
		}
		if got := r.URL.Query().Get("wallet"); got != "0xabc" {
			t.Fatalf("wallet query = %s", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer srv.Close()

	invoker := NewHTTPInvoker(HTTPInvokerConfig{BaseURL: srv.URL, InternalToken: "token"})
	result, err := invoker.Invoke(context.Background(), &ginai.Tool{Method: "GET", Path: "/"}, map[string]any{"wallet": "0xabc"})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if result.StatusCode != 200 {
		t.Fatalf("status = %d", result.StatusCode)
	}
}

func TestHTTPInvokerPOST(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if payload["value"] != "Paer" {
			t.Fatalf("value = %v", payload["value"])
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"updated": true})
	}))
	defer srv.Close()

	invoker := NewHTTPInvoker(HTTPInvokerConfig{BaseURL: srv.URL, InternalToken: "token"})
	result, err := invoker.Invoke(context.Background(), &ginai.Tool{Method: "POST", Path: "/"}, map[string]any{"value": "Paer"})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if result.StatusCode != 200 {
		t.Fatalf("status = %d", result.StatusCode)
	}
}
