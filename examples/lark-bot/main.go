package main

import (
	"context"
	"github.com/paerx/gin.agent/pkg/adapter/lark"
	"github.com/paerx/gin.agent/pkg/agent"
	"github.com/paerx/gin.agent/pkg/audit"
	"github.com/paerx/gin.agent/pkg/auth"
	"github.com/paerx/gin.agent/pkg/config"
	"github.com/paerx/gin.agent/pkg/ginai"
	"github.com/paerx/gin.agent/pkg/storage"
	"github.com/paerx/gin.agent/pkg/transport"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
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

type memorySender struct{}

func (s *memorySender) SendText(_ context.Context, _ string, text string) error {
	println("[lark reply]", text)
	return nil
}

func main() {
	cfg := config.LoadFromEnv()

	registry := ginai.NewRegistry()
	ginAgent := ginai.NewBinder(registry)
	registerDemoTools(registry)
	memoryStore := buildMemoryStore(cfg)

	auditStore, err := storage.NewSQLiteAuditStore(cfg.Audit.DSN)
	if err != nil {
		panic(err)
	}

	planner := buildPlanner(cfg)
	invoker := transport.NewHTTPInvoker(transport.HTTPInvokerConfig{
		BaseURL:       cfg.GinAI.BaseURL,
		InternalToken: cfg.GinAI.InternalToken,
	})
	roleResolver := agent.StaticRoleResolver{
		"admin-user":    {"owner", "admin", "operator", "readonly"},
		"operator-user": {"operator", "readonly"},
		"ea1g74bc":      {"owner", "admin", "operator", "readonly"},
	}
	if ownerUserID := os.Getenv("GINAI_OWNER_USER_ID"); ownerUserID != "" {
		roleResolver[ownerUserID] = []string{"owner", "admin", "operator", "readonly"}
	}
	roleStore := agent.NewMemoryRoleStore(roleResolver)

	botAgent := agent.New(agent.Config{
		Registry:           registry,
		Memory:             memoryStore,
		Planner:            planner,
		Invoker:            invoker,
		PermissionChecker:  auth.NewStaticChecker(),
		Formatter:          agent.NewTextFormatter(),
		AuditStore:         auditStore,
		RoleResolver:       roleStore,
		MaxContextMessages: cfg.GinAI.MaxContextMessages,
		MaxContextChars:    cfg.GinAI.MaxContextChars,
	})

	httpSender := lark.NewHTTPSender(lark.HTTPSenderConfig{
		AppID:     cfg.Lark.AppID,
		AppSecret: cfg.Lark.AppSecret,
		BaseURL:   cfg.Lark.Domain,
	})
	sender := chooseSender(cfg, httpSender)
	bot := lark.NewBot(botAgent, sender, cfg.Lark.VerificationToken, cfg.Lark.EncryptKey)
	wsBot := lark.NewLongConnectionBotWithDomain(botAgent, sender, cfg.Lark.AppID, cfg.Lark.AppSecret, cfg.Lark.VerificationToken, cfg.Lark.EncryptKey, cfg.Lark.Domain)

	router := gin.Default()
	router.Use(internalTokenMiddleware(cfg.GinAI.InternalToken))
	ginai.RegisterDebugRoutes(router, registry)
	router.POST("/lark/events", bot.HandleEvent)

	api := router.Group("/api")
	api.GET("/getUserinfo", ginAgent.Bind(*getUserInfoTool()), getUserInfoHandler)
	api.POST("/updateUserinfo", ginAgent.Bind(*updateUserInfoTool()), updateUserInfoHandler)

	if cfg.Lark.EventMode == "ws" {
		go func() {
			if err := wsBot.Start(context.Background()); err != nil {
				panic(err)
			}
		}()
	}

	if err := router.Run(cfg.Server.Addr); err != nil {
		panic(err)
	}
}

func registerDemoTools(registry *ginai.Registry) {
	if err := registry.Register(getUserInfoTool()); err != nil {
		panic(err)
	}
	if err := registry.Register(updateUserInfoTool()); err != nil {
		panic(err)
	}
}

func getUserInfoTool() *ginai.Tool {
	return &ginai.Tool{
		Name:        "get_user_info",
		Description: "查询用户信息，支持通过钱包地址或用户ID查询",
		Method:      "GET",
		Path:        "/api/getUserinfo",
		Params:      getUserInfoReq{},
		ReadOnly:    true,
	}
}

func updateUserInfoTool() *ginai.Tool {
	return &ginai.Tool{
		Name:        "update_user_info",
		Description: "更新用户基础信息，只允许更新 nickname、avatar、bio",
		Method:      "POST",
		Path:        "/api/updateUserinfo",
		Params:      updateUserInfoReq{},
		ReadOnly:    false,
		NeedConfirm: true,
		Roles:       []string{"admin", "operator"},
		AllowFields: []string{"nickname", "avatar", "bio"},
	}
}

func buildPlanner(cfg config.Config) agent.Planner {
	if strings.TrimSpace(cfg.LLM.APIKey) == "" {
		return agent.NewRulePlanner()
	}
	return agent.NewOpenAIPlanner(agent.OpenAIPlannerConfig{
		BaseURL: cfg.LLM.BaseURL,
		APIKey:  cfg.LLM.APIKey,
		Model:   cfg.LLM.Model,
		Timeout: cfg.LLM.Timeout,
	})
}

func chooseSender(cfg config.Config, sender lark.Sender) lark.Sender {
	if cfg.Lark.AppID == "" || cfg.Lark.AppSecret == "" {
		return &memorySender{}
	}
	return sender
}

func buildMemoryStore(cfg config.Config) agent.MemoryStore {
	if os.Getenv("GINAI_MEMORY_STORE") == "memory" {
		return storage.NewMemoryStore(cfg.GinAI.ContextTTL, cfg.GinAI.MaxHistory)
	}
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	return storage.NewRedisMemoryStore(storage.RedisMemoryConfig{
		Client:     rdb,
		TTL:        cfg.GinAI.ContextTTL,
		MaxHistory: cfg.GinAI.MaxHistory,
	})
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

func getUserInfoHandler(c *gin.Context) {
	wallet := c.Query("wallet")
	if wallet == "" {
		wallet = "0xabc"
	}
	userID := c.Query("user_id")
	if userID == "" {
		userID = "1001"
	}
	c.JSON(http.StatusOK, gin.H{
		"user_id":    userID,
		"wallet":     wallet,
		"nickname":   "Tom",
		"points":     1200,
		"status":     "normal",
		"created_at": "2026-05-01T10:00:00Z",
	})
}

func updateUserInfoHandler(c *gin.Context) {
	var req updateUserInfoReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"wallet":  req.Wallet,
		"field":   req.Field,
		"value":   req.Value,
		"updated": true,
	})
}

var _ audit.Store = (*storage.SQLiteAuditStore)(nil)
