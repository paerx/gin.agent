package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"gin.agent/pkg/ginai"
)

type Planner interface {
	Plan(ctx context.Context, input PlannerInput) (*PlanResult, error)
}

type PlannerInput struct {
	UserText     string                `json:"user_text"`
	Messages     []Message             `json:"messages"`
	State        SessionState          `json:"state"`
	Tools        []ginai.LLMToolSchema `json:"tools"`
	SystemPrompt string                `json:"system_prompt"`
}

type OpenAIPlannerConfig struct {
	BaseURL      string
	APIKey       string
	Model        string
	SystemPrompt string
	Timeout      time.Duration
	HTTPClient   *http.Client
}

type OpenAIPlanner struct {
	baseURL      string
	apiKey       string
	model        string
	systemPrompt string
	client       *http.Client
}

func NewOpenAIPlanner(cfg OpenAIPlannerConfig) *OpenAIPlanner {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: timeout}
	}
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	prompt := cfg.SystemPrompt
	if prompt == "" {
		prompt = DefaultSystemPrompt
	}
	return &OpenAIPlanner{
		baseURL:      baseURL,
		apiKey:       cfg.APIKey,
		model:        cfg.Model,
		systemPrompt: prompt,
		client:       client,
	}
}

func (p *OpenAIPlanner) Plan(ctx context.Context, input PlannerInput) (*PlanResult, error) {
	type message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	type responseFormat struct {
		Type string `json:"type"`
	}
	body := map[string]any{
		"model": p.model,
		"messages": []message{
			{Role: "system", Content: input.SystemPrompt},
			{Role: "user", Content: buildPlannerPayload(input)},
		},
		"response_format": responseFormat{Type: "json_object"},
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, err
	}
	if len(parsed.Choices) == 0 {
		return nil, fmt.Errorf("planner returned no choices")
	}
	var result PlanResult
	if err := json.Unmarshal([]byte(parsed.Choices[0].Message.Content), &result); err != nil {
		return nil, fmt.Errorf("invalid planner output: %w", err)
	}
	return normalizePlanResult(&result)
}

func buildPlannerPayload(input PlannerInput) string {
	payload := map[string]any{
		"messages":      input.Messages,
		"session_state": input.State,
		"tools":         input.Tools,
		"user_input":    input.UserText,
	}
	raw, _ := json.Marshal(payload)
	return string(raw)
}

func normalizePlanResult(result *PlanResult) (*PlanResult, error) {
	switch result.Type {
	case "text", "ask", "refuse":
		if strings.TrimSpace(result.Text) == "" {
			return nil, fmt.Errorf("planner text output is empty")
		}
	case "tool_call":
		if result.ToolCall == nil || strings.TrimSpace(result.ToolCall.ToolName) == "" {
			return nil, fmt.Errorf("planner tool call is missing tool_name")
		}
		if result.ToolCall.Arguments == nil {
			result.ToolCall.Arguments = map[string]any{}
		}
	default:
		return nil, fmt.Errorf("unsupported planner result type %q", result.Type)
	}
	return result, nil
}
