package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	larkadapter "gin.agent/pkg/adapter/lark"
	"gin.agent/pkg/agent"
	"gin.agent/pkg/auth"
	"gin.agent/pkg/ginai"
	"gin.agent/pkg/storage"
	"gin.agent/pkg/transport"

	"github.com/gin-gonic/gin"
)

type getUserInfoReq struct {
	Wallet string `json:"wallet" ai:"desc=用户钱包地址"`
	UserID string `json:"user_id" ai:"desc=用户ID"`
}

type updateUserInfoReq struct {
	Wallet string `json:"wallet" ai:"desc=用户钱包地址,required"`
	Field  string `json:"field" ai:"desc=要更新的字段,enum=nickname|avatar|bio,required"`
	Value  string `json:"value" ai:"desc=新的字段值,required"`
}

type captureSender struct {
	mu       sync.Mutex
	messages []string
}

func (s *captureSender) SendText(_ context.Context, _ string, text string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = append(s.messages, text)
	return nil
}

func (s *captureSender) last() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.messages) == 0 {
		return ""
	}
	return s.messages[len(s.messages)-1]
}

func TestLarkWebhookEndToEnd(t *testing.T) {
	gin.SetMode(gin.TestMode)
	registry := ginai.NewRegistry()
	binder := ginai.NewBinder(registry)
	memoryStore := storage.NewMemoryStore(time.Hour, 30)
	auditStore, err := storage.NewSQLiteAuditStore("file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("NewSQLiteAuditStore() error = %v", err)
	}
	sender := &captureSender{}

	router := gin.New()
	router.Use(internalTokenMiddleware("test-token"))
	ginai.RegisterDebugRoutes(router, registry)

	api := router.Group("/api")
	api.GET("/getUserinfo", binder.Bind(ginai.Tool{
		Name:        "get_user_info",
		Description: "查询用户信息",
		Params:      getUserInfoReq{},
		ReadOnly:    true,
	}), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"wallet":   c.Query("wallet"),
			"nickname": "Tom",
			"points":   1200,
		})
	})
	api.POST("/updateUserinfo", binder.Bind(ginai.Tool{
		Name:        "update_user_info",
		Description: "更新用户基础信息",
		Params:      updateUserInfoReq{},
		ReadOnly:    false,
		NeedConfirm: true,
		Roles:       []string{"operator"},
		AllowFields: []string{"nickname", "avatar", "bio"},
	}), func(c *gin.Context) {
		var req updateUserInfoReq
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"wallet": req.Wallet, "field": req.Field, "value": req.Value, "updated": true})
	})

	srv := httptest.NewServer(router)
	defer srv.Close()

	botAgent := agent.New(agent.Config{
		Registry: registry,
		Memory:   memoryStore,
		Planner:  agent.NewRulePlanner(),
		Invoker: transport.NewHTTPInvoker(transport.HTTPInvokerConfig{
			BaseURL:       srv.URL,
			InternalToken: "test-token",
		}),
		PermissionChecker: auth.NewStaticChecker(),
		Formatter:         agent.NewTextFormatter(),
		AuditStore:        auditStore,
		RoleResolver:      agent.StaticRoleResolver{"operator-user": {"operator", "readonly"}},
	})
	bot := larkadapter.NewBot(botAgent, sender, "verify-token", "")
	router.POST("/lark/events", bot.HandleEvent)

	registerTools(t, srv.URL)
	postLarkMessage(t, router, "msg-1", "operator-user", "查一下 0xabc 用户信息")
	if got := sender.last(); !strings.Contains(got, "Tom") {
		t.Fatalf("query reply = %q", got)
	}

	postLarkMessage(t, router, "msg-2", "operator-user", "把他的昵称改成 Paer")
	if got := sender.last(); !strings.Contains(got, "回复“确认”执行") {
		t.Fatalf("confirm prompt = %q", got)
	}

	postLarkMessage(t, router, "msg-3", "operator-user", "确认")
	if got := sender.last(); !strings.Contains(got, "updated") && !strings.Contains(got, "true") {
		t.Fatalf("confirm reply = %q", got)
	}

	count, err := auditStore.Count(context.Background())
	if err != nil {
		t.Fatalf("audit count error = %v", err)
	}
	if count < 3 {
		t.Fatalf("audit count = %d", count)
	}
}

func registerTools(t *testing.T, baseURL string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, baseURL+"/api/getUserinfo?wallet=0xabc", nil)
	if err != nil {
		t.Fatalf("new get request: %v", err)
	}
	req.Header.Set("X-GinAI-Internal-Token", "test-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("register get tool request: %v", err)
	}
	_ = resp.Body.Close()

	raw := bytes.NewBufferString(`{"wallet":"0xabc","field":"nickname","value":"Tom"}`)
	req, err = http.NewRequest(http.MethodPost, baseURL+"/api/updateUserinfo", raw)
	if err != nil {
		t.Fatalf("new post request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GinAI-Internal-Token", "test-token")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("register update tool request: %v", err)
	}
	_ = resp.Body.Close()
}

func postLarkMessage(t *testing.T, router http.Handler, messageID, userID, text string) {
	t.Helper()
	payload := map[string]any{
		"header": map[string]any{
			"token":      "verify-token",
			"event_type": "im.message.receive_v1",
		},
		"event": map[string]any{
			"sender": map[string]any{
				"sender_id": map[string]any{"user_id": userID},
			},
			"message": map[string]any{
				"message_id":   messageID,
				"chat_id":      "chat-1",
				"chat_type":    "p2p",
				"message_type": "text",
				"content":      `{"text":"` + text + `"}`,
			},
		},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/lark/events", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("webhook status = %d body = %s", w.Code, w.Body.String())
	}
}

func internalTokenMiddleware(token string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.URL.Path == "/lark/events" || c.Request.URL.Path == "/debug/ginai/tools" {
			c.Next()
			return
		}
		if strings.HasPrefix(c.FullPath(), "/api/") && c.Request.Header.Get("X-GinAI-Internal-Token") != token {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid internal token"})
			return
		}
		c.Next()
	}
}
