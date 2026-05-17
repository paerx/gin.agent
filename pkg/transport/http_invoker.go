package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"gin.agent/pkg/ginai"
)

type Invoker interface {
	Invoke(ctx context.Context, tool *ginai.Tool, args map[string]any) (*InvokeResult, error)
}

type InvokeResult struct {
	StatusCode int    `json:"status_code"`
	RawBody    []byte `json:"raw_body"`
	Summary    string `json:"summary"`
}

type HTTPInvokerConfig struct {
	BaseURL       string
	InternalToken string
	Timeout       time.Duration
}

type HTTPInvoker struct {
	baseURL       string
	internalToken string
	client        *http.Client
}

func NewHTTPInvoker(cfg HTTPInvokerConfig) *HTTPInvoker {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &HTTPInvoker{
		baseURL:       strings.TrimRight(cfg.BaseURL, "/"),
		internalToken: cfg.InternalToken,
		client:        &http.Client{Timeout: timeout},
	}
}

func (i *HTTPInvoker) Invoke(ctx context.Context, tool *ginai.Tool, args map[string]any) (*InvokeResult, error) {
	method := strings.ToUpper(tool.Method)
	if method == http.MethodDelete {
		return nil, fmt.Errorf("delete is forbidden")
	}

	target := i.baseURL + tool.Path
	var body io.Reader
	if method == http.MethodGet {
		values := url.Values{}
		for key, value := range args {
			values.Set(key, fmt.Sprintf("%v", value))
		}
		if encoded := values.Encode(); encoded != "" {
			target += "?" + encoded
		}
	} else {
		payload, err := json.Marshal(args)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-GinAI-Internal-Token", i.internalToken)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := i.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	summary := string(raw)
	if len(summary) > 200 {
		summary = summary[:200] + "..."
	}
	return &InvokeResult{StatusCode: resp.StatusCode, RawBody: raw, Summary: summary}, nil
}
