package agent_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/paerx/gin.agent/pkg/agent"
	"github.com/paerx/gin.agent/pkg/auth"
	"github.com/paerx/gin.agent/pkg/ginai"
	"github.com/paerx/gin.agent/pkg/storage"
	"github.com/paerx/gin.agent/pkg/transport"

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
		RoleResolver:      agent.StaticRoleResolver{"user1": {"operator", "readonly"}},
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
		RoleResolver:      agent.StaticRoleResolver{"user1": {"operator", "readonly"}},
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

func TestAgentCleanClearsOnlyCurrentConversation(t *testing.T) {
	store := storage.NewMemoryStore(time.Hour, 30)
	registry := ginai.NewRegistry()
	agt := agent.New(agent.Config{
		Registry:          registry,
		Memory:            store,
		Planner:           agent.NewRulePlanner(),
		Invoker:           transport.NewHTTPInvoker(transport.HTTPInvokerConfig{BaseURL: "http://127.0.0.1:1"}),
		PermissionChecker: auth.NewStaticChecker(),
		Formatter:         agent.NewTextFormatter(),
		RoleResolver:      agent.StaticRoleResolver{"user1": {"readonly"}, "user2": {"readonly"}},
	})

	ctx := context.Background()
	user1Conversation := agent.BuildConversationID("lark", "group", "chat1", "user1")
	user2Conversation := agent.BuildConversationID("lark", "group", "chat1", "user2")
	_ = store.AppendMessage(ctx, user1Conversation, agent.Message{Role: "user", Content: "u1"})
	_ = store.SetState(ctx, user1Conversation, agent.SessionState{LastUserWallet: "0xabc"})
	_ = store.AppendMessage(ctx, user2Conversation, agent.Message{Role: "user", Content: "u2"})
	_ = store.SetState(ctx, user2Conversation, agent.SessionState{LastUserWallet: "0xdef"})

	out, err := agt.HandleMessage(ctx, agent.AgentInput{
		Platform:  "lark",
		ChatID:    "chat1",
		ChatType:  "group",
		UserID:    "user1",
		MessageID: "clean-msg",
		Text:      "clean",
	})
	if err != nil {
		t.Fatalf("HandleMessage(clean) error = %v", err)
	}
	if out.Text != "已清空你的上下文。" {
		t.Fatalf("clean output = %s", out.Text)
	}

	user1State, _ := store.GetState(ctx, user1Conversation)
	if user1State.LastUserWallet != "" {
		t.Fatalf("user1 state still present: %#v", user1State)
	}
	user2State, _ := store.GetState(ctx, user2Conversation)
	if user2State.LastUserWallet != "0xdef" {
		t.Fatalf("user2 state was affected: %#v", user2State)
	}
}

func TestOwnerCanManageUserRoles(t *testing.T) {
	store := storage.NewMemoryStore(time.Hour, 30)
	roleStore := agent.NewMemoryRoleStore(map[string][]string{
		"owner1": {"owner", "admin", "operator", "readonly"},
	})
	registry := ginai.NewRegistry()
	agt := agent.New(agent.Config{
		Registry:          registry,
		Memory:            store,
		Planner:           agent.NewRulePlanner(),
		Invoker:           transport.NewHTTPInvoker(transport.HTTPInvokerConfig{BaseURL: "http://127.0.0.1:1"}),
		PermissionChecker: auth.NewStaticChecker(),
		Formatter:         agent.NewTextFormatter(),
		RoleResolver:      roleStore,
	})

	out, err := agt.HandleMessage(context.Background(), agent.AgentInput{
		Platform:  "lark",
		ChatID:    "chat1",
		ChatType:  "group",
		UserID:    "owner1",
		MessageID: "role-msg-1",
		Text:      "add user user2 operator readonly",
	})
	if err != nil {
		t.Fatalf("HandleMessage(add user) error = %v", err)
	}
	if !strings.Contains(out.Text, "operator") {
		t.Fatalf("add user output = %s", out.Text)
	}

	roles, err := roleStore.Resolve(context.Background(), "lark", "user2")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if !containsRole(roles, "operator") || !containsRole(roles, "readonly") {
		t.Fatalf("roles = %#v", roles)
	}
}

func TestUserCanRequestRolesAndOwnerCanApprove(t *testing.T) {
	store := storage.NewMemoryStore(time.Hour, 30)
	roleStore := agent.NewMemoryRoleStore(map[string][]string{
		"owner1": {"owner", "admin", "operator", "readonly"},
	})
	agt := agent.New(agent.Config{
		Registry:          ginai.NewRegistry(),
		Memory:            store,
		Planner:           agent.NewRulePlanner(),
		Invoker:           transport.NewHTTPInvoker(transport.HTTPInvokerConfig{BaseURL: "http://127.0.0.1:1"}),
		PermissionChecker: auth.NewStaticChecker(),
		Formatter:         agent.NewTextFormatter(),
		RoleResolver:      roleStore,
	})

	out, err := agt.HandleMessage(context.Background(), agent.AgentInput{
		Platform:    "lark",
		ChatID:      "chat1",
		ChatType:    "group",
		UserID:      "user2",
		DisplayName: "Paer",
		MessageID:   "request-role-msg",
		Text:        "addme operator readonly",
	})
	if err != nil {
		t.Fatalf("HandleMessage(addme) error = %v", err)
	}
	if !strings.Contains(out.Text, "已提交权限申请") || !strings.Contains(out.Text, "Paer") {
		t.Fatalf("addme output = %s", out.Text)
	}

	requests, err := roleStore.ListRoleRequests(context.Background())
	if err != nil {
		t.Fatalf("ListRoleRequests() error = %v", err)
	}
	if len(requests) != 1 {
		t.Fatalf("request count = %d", len(requests))
	}

	out, err = agt.HandleMessage(context.Background(), agent.AgentInput{
		Platform:  "lark",
		ChatID:    "chat1",
		ChatType:  "group",
		UserID:    "owner1",
		MessageID: "approve-role-msg",
		Text:      "approve " + requests[0].ID,
	})
	if err != nil {
		t.Fatalf("HandleMessage(approve) error = %v", err)
	}
	if !strings.Contains(out.Text, "已批准") {
		t.Fatalf("approve output = %s", out.Text)
	}

	roles, err := roleStore.Resolve(context.Background(), "lark", "user2")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if !containsRole(roles, "operator") || !containsRole(roles, "readonly") {
		t.Fatalf("roles = %#v", roles)
	}
}

func TestMyUserIDCommand(t *testing.T) {
	agt := agent.New(agent.Config{
		Registry:          ginai.NewRegistry(),
		Memory:            storage.NewMemoryStore(time.Hour, 30),
		Planner:           agent.NewRulePlanner(),
		Invoker:           transport.NewHTTPInvoker(transport.HTTPInvokerConfig{BaseURL: "http://127.0.0.1:1"}),
		PermissionChecker: auth.NewStaticChecker(),
		Formatter:         agent.NewTextFormatter(),
		RoleResolver:      agent.StaticRoleResolver{"user1": {"readonly"}},
	})

	out, err := agt.HandleMessage(context.Background(), agent.AgentInput{
		Platform:    "lark",
		ChatID:      "chat1",
		ChatType:    "group",
		UserID:      "user1",
		DisplayName: "Paer",
		MessageID:   "my-user-id-msg",
		Text:        "myuserid",
	})
	if err != nil {
		t.Fatalf("HandleMessage(myuserid) error = %v", err)
	}
	if !strings.Contains(out.Text, "user1") || !strings.Contains(out.Text, "Paer") {
		t.Fatalf("myuserid output = %s", out.Text)
	}
}

func TestAgentWarnsWhenContextTooLong(t *testing.T) {
	store := storage.NewMemoryStore(time.Hour, 30)
	registry := ginai.NewRegistry()
	agt := agent.New(agent.Config{
		Registry:           registry,
		Memory:             store,
		Planner:            agent.NewRulePlanner(),
		Invoker:            transport.NewHTTPInvoker(transport.HTTPInvokerConfig{BaseURL: "http://127.0.0.1:1"}),
		PermissionChecker:  auth.NewStaticChecker(),
		Formatter:          agent.NewTextFormatter(),
		RoleResolver:       agent.StaticRoleResolver{"user1": {"readonly"}},
		MaxContextMessages: 2,
		MaxContextChars:    1000,
	})

	out, err := agt.HandleMessage(context.Background(), agent.AgentInput{
		Platform:  "lark",
		ChatID:    "chat1",
		ChatType:  "p2p",
		UserID:    "user1",
		MessageID: "long-context-1",
		Text:      "这是一条普通消息",
	})
	if err != nil {
		t.Fatalf("HandleMessage first error = %v", err)
	}
	if out == nil || out.Text == "" {
		t.Fatal("expected first output")
	}

	out, err = agt.HandleMessage(context.Background(), agent.AgentInput{
		Platform:  "lark",
		ChatID:    "chat1",
		ChatType:  "p2p",
		UserID:    "user1",
		MessageID: "long-context-2",
		Text:      "这是第二条普通消息",
	})
	if err != nil {
		t.Fatalf("HandleMessage second error = %v", err)
	}
	if !strings.Contains(out.Text, "上下文") || !strings.Contains(out.Text, "clean") {
		t.Fatalf("long context output = %s", out.Text)
	}

	out, err = agt.HandleMessage(context.Background(), agent.AgentInput{
		Platform:  "lark",
		ChatID:    "chat1",
		ChatType:  "p2p",
		UserID:    "user1",
		MessageID: "long-context-clean",
		Text:      "clean",
	})
	if err != nil {
		t.Fatalf("HandleMessage clean error = %v", err)
	}
	if out.Text != "已清空你的上下文。" {
		t.Fatalf("clean output = %s", out.Text)
	}
}

func containsRole(roles []string, role string) bool {
	for _, item := range roles {
		if item == role {
			return true
		}
	}
	return false
}

type plannerFunc func(context.Context, agent.PlannerInput) (*agent.PlanResult, error)

func (f plannerFunc) Plan(ctx context.Context, input agent.PlannerInput) (*agent.PlanResult, error) {
	return f(ctx, input)
}
