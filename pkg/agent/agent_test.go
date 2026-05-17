package agent_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gin.agent/pkg/agent"
	"gin.agent/pkg/auth"
	"gin.agent/pkg/ginai"
	"gin.agent/pkg/storage"
	"gin.agent/pkg/transport"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestAgentConfirmFlow(t *testing.T) {
	registry := ginai.NewRegistry()
	if err := registry.Register(&ginai.Tool{
		Name:        "get_user_info",
		Description: "查询用户信息",
		Method:      "GET",
		Path:        "/get",
		Params: struct {
			Wallet string `json:"wallet"`
		}{},
		ReadOnly: true,
	}); err != nil {
		t.Fatalf("register get tool: %v", err)
	}
	if err := registry.Register(&ginai.Tool{
		Name:        "update_user_info",
		Description: "更新用户信息",
		Method:      "POST",
		Path:        "/update",
		Params: struct {
			Wallet string `json:"wallet"`
			Field  string `json:"field"`
			Value  string `json:"value"`
		}{},
		ReadOnly:    false,
		NeedConfirm: true,
		Roles:       []string{"operator"},
		AllowFields: []string{"nickname"},
	}); err != nil {
		t.Fatalf("register update tool: %v", err)
	}

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run(): %v", err)
	}
	defer mr.Close()
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	memoryStore := storage.NewRedisMemoryStore(storage.RedisMemoryConfig{
		Client:     client,
		TTL:        time.Hour,
		MaxHistory: 30,
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/get":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"wallet":   r.URL.Query().Get("wallet"),
				"nickname": "Tom",
			})
		case "/update":
			var payload map[string]any
			_ = json.NewDecoder(r.Body).Decode(&payload)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"wallet":  payload["wallet"],
				"updated": true,
			})
		}
	}))
	defer srv.Close()

	agt := agent.New(agent.Config{
		Registry:          registry,
		Memory:            memoryStore,
		Planner:           agent.NewRulePlanner(),
		Invoker:           transport.NewHTTPInvoker(transport.HTTPInvokerConfig{BaseURL: srv.URL, InternalToken: "token"}),
		PermissionChecker: auth.NewStaticChecker(),
		Formatter:         agent.NewTextFormatter(),
		RoleResolver:      agent.StaticRoleResolver{"user1": {"operator"}},
		ConfirmTTL:        5 * time.Minute,
	})

	out, err := agt.HandleMessage(context.Background(), agent.AgentInput{
		Platform:  "lark",
		ChatID:    "chat1",
		ChatType:  "p2p",
		UserID:    "user1",
		MessageID: "msg1",
		Text:      "查一下 0xabc 用户信息",
	})
	if err != nil {
		t.Fatalf("HandleMessage() query error = %v", err)
	}
	if out == nil || out.Text == "" {
		t.Fatal("expected query output")
	}

	out, err = agt.HandleMessage(context.Background(), agent.AgentInput{
		Platform:  "lark",
		ChatID:    "chat1",
		ChatType:  "p2p",
		UserID:    "user1",
		MessageID: "msg2",
		Text:      "把他的昵称改成 Paer",
	})
	if err != nil {
		t.Fatalf("HandleMessage() write error = %v", err)
	}
	if out == nil || out.Text == "" || out.Text == "已更新。" {
		t.Fatalf("expected confirmation message, got %v", out)
	}

	out, err = agt.HandleMessage(context.Background(), agent.AgentInput{
		Platform:  "lark",
		ChatID:    "chat1",
		ChatType:  "p2p",
		UserID:    "user1",
		MessageID: "msg3",
		Text:      "确认",
	})
	if err != nil {
		t.Fatalf("HandleMessage() confirm error = %v", err)
	}
	if out == nil || out.Text == "" {
		t.Fatal("expected confirm output")
	}
}

func TestAgentRejectsInvalidToolArguments(t *testing.T) {
	registry := ginai.NewRegistry()
	if err := registry.Register(&ginai.Tool{
		Name:        "update_user_info",
		Description: "更新用户信息",
		Method:      "POST",
		Path:        "/update",
		Params: struct {
			Wallet string `json:"wallet" ai:"required"`
			Field  string `json:"field" ai:"enum=nickname|avatar,required"`
			Value  string `json:"value" ai:"required"`
		}{},
		ReadOnly:    false,
		NeedConfirm: true,
		Roles:       []string{"operator"},
		AllowFields: []string{"nickname", "avatar"},
	}); err != nil {
		t.Fatalf("register update tool: %v", err)
	}

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run(): %v", err)
	}
	defer mr.Close()
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	agt := agent.New(agent.Config{
		Registry: registry,
		Memory: storage.NewRedisMemoryStore(storage.RedisMemoryConfig{
			Client:     client,
			TTL:        time.Hour,
			MaxHistory: 30,
		}),
		Planner: plannerFunc(func(context.Context, agent.PlannerInput) (*agent.PlanResult, error) {
			return &agent.PlanResult{
				Type: "tool_call",
				ToolCall: &agent.ToolCall{
					ToolName: "update_user_info",
					Arguments: map[string]any{
						"wallet": "0xabc",
						"field":  "role",
						"value":  "admin",
					},
				},
			}, nil
		}),
		Invoker:           transport.NewHTTPInvoker(transport.HTTPInvokerConfig{BaseURL: "http://127.0.0.1:1"}),
		PermissionChecker: auth.NewStaticChecker(),
		Formatter:         agent.NewTextFormatter(),
		RoleResolver:      agent.StaticRoleResolver{"user1": {"operator"}},
		ConfirmTTL:        5 * time.Minute,
	})

	out, err := agt.HandleMessage(context.Background(), agent.AgentInput{
		Platform:  "lark",
		ChatID:    "chat1",
		ChatType:  "p2p",
		UserID:    "user1",
		MessageID: "invalid-msg",
		Text:      "把他的角色改成 admin",
	})
	if err != nil {
		t.Fatalf("HandleMessage() error = %v", err)
	}
	if out.Text != "参数格式不正确，请检查后重试。" {
		t.Fatalf("unexpected output = %s", out.Text)
	}
}

type plannerFunc func(context.Context, agent.PlannerInput) (*agent.PlanResult, error)

func (f plannerFunc) Plan(ctx context.Context, input agent.PlannerInput) (*agent.PlanResult, error) {
	return f(ctx, input)
}
