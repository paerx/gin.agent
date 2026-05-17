package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Server struct {
		Addr string
	}
	LLM struct {
		Provider string
		BaseURL  string
		Model    string
		APIKey   string
		Timeout  time.Duration
	}
	Lark struct {
		AppID             string
		AppSecret         string
		VerificationToken string
		EncryptKey        string
		EventMode         string
		Domain            string
	}
	Redis struct {
		Addr     string
		Password string
		DB       int
	}
	Audit struct {
		Driver string
		DSN    string
	}
	GinAI struct {
		BaseURL       string
		InternalToken string
		ContextTTL    time.Duration
		MaxHistory    int64
	}
}

func LoadFromEnv() Config {
	var cfg Config
	cfg.Server.Addr = env("SERVER_ADDR", ":8080")
	cfg.LLM.Provider = env("LLM_PROVIDER", "openai")
	cfg.LLM.BaseURL = env("LLM_BASE_URL", "https://api.openai.com/v1")
	cfg.LLM.Model = env("LLM_MODEL", "gpt-4.1")
	cfg.LLM.APIKey = os.Getenv("OPENAI_API_KEY")
	cfg.LLM.Timeout = envDuration("LLM_TIMEOUT", 20*time.Second)
	cfg.Lark.AppID = os.Getenv("LARK_APP_ID")
	cfg.Lark.AppSecret = os.Getenv("LARK_APP_SECRET")
	cfg.Lark.VerificationToken = os.Getenv("LARK_VERIFICATION_TOKEN")
	cfg.Lark.EncryptKey = os.Getenv("LARK_ENCRYPT_KEY")
	cfg.Lark.EventMode = env("LARK_EVENT_MODE", "http")
	cfg.Lark.Domain = env("LARK_DOMAIN", "https://open.feishu.cn")
	cfg.Redis.Addr = env("REDIS_ADDR", "localhost:6379")
	cfg.Redis.Password = os.Getenv("REDIS_PASSWORD")
	cfg.Redis.DB = envInt("REDIS_DB", 0)
	cfg.Audit.Driver = env("AUDIT_DRIVER", "sqlite")
	cfg.Audit.DSN = env("AUDIT_DSN", "file:ginai_audit.db")
	cfg.GinAI.BaseURL = env("GINAI_BASE_URL", "http://localhost:8080")
	cfg.GinAI.InternalToken = env("GINAI_INTERNAL_TOKEN", "dev-internal-token")
	cfg.GinAI.ContextTTL = envDuration("GINAI_CONTEXT_TTL", 168*time.Hour)
	cfg.GinAI.MaxHistory = int64(envInt("GINAI_MAX_HISTORY", 30))
	return cfg
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envInt(key string, fallback int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

func envDuration(key string, fallback time.Duration) time.Duration {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	value, err := time.ParseDuration(raw)
	if err != nil {
		return fallback
	}
	return value
}
