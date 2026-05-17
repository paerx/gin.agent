package lark

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type HTTPSenderConfig struct {
	AppID     string
	AppSecret string
	BaseURL   string
	Client    *http.Client
}

type HTTPSender struct {
	appID     string
	appSecret string
	baseURL   string
	client    *http.Client
}

func NewHTTPSender(cfg HTTPSenderConfig) *HTTPSender {
	client := cfg.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://open.feishu.cn/open-apis"
	} else {
		baseURL = strings.TrimRight(baseURL, "/")
		if !strings.HasSuffix(baseURL, "/open-apis") {
			baseURL += "/open-apis"
		}
	}
	return &HTTPSender{
		appID:     cfg.AppID,
		appSecret: cfg.AppSecret,
		baseURL:   baseURL,
		client:    client,
	}
}

func (s *HTTPSender) SendText(ctx context.Context, chatID string, text string) error {
	token, err := s.fetchTenantAccessToken(ctx)
	if err != nil {
		return err
	}
	payload := map[string]any{
		"receive_id": chatID,
		"msg_type":   "text",
		"content":    fmt.Sprintf(`{"text":%q}`, text),
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/im/v1/messages?receive_id_type=chat_id", bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("send lark message failed: %s: %s", resp.Status, strings.TrimSpace(string(raw)))
	}
	return nil
}

func (s *HTTPSender) fetchTenantAccessToken(ctx context.Context) (string, error) {
	payload := map[string]string{
		"app_id":     s.appID,
		"app_secret": s.appSecret,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/auth/v3/tenant_access_token/internal", bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, err = io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var parsed struct {
		TenantAccessToken string `json:"tenant_access_token"`
		Code              int    `json:"code"`
		Msg               string `json:"msg"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", err
	}
	if parsed.Code != 0 || parsed.TenantAccessToken == "" {
		return "", fmt.Errorf("fetch tenant access token failed: %s: %s", parsed.Msg, strings.TrimSpace(string(raw)))
	}
	return parsed.TenantAccessToken, nil
}
