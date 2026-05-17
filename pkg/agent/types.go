package agent

type AgentInput struct {
	Platform  string `json:"platform"`
	ChatID    string `json:"chat_id"`
	ChatType  string `json:"chat_type"`
	UserID    string `json:"user_id"`
	MessageID string `json:"message_id"`
	Text      string `json:"text"`
}

type AgentOutput struct {
	Text string `json:"text"`
}

type PlanResult struct {
	Type     string    `json:"type"`
	Text     string    `json:"text"`
	ToolCall *ToolCall `json:"tool_call,omitempty"`
}

type ToolCall struct {
	ToolName  string         `json:"tool_name"`
	Arguments map[string]any `json:"arguments"`
}

type Message struct {
	Role      string `json:"role"`
	Content   string `json:"content"`
	ToolName  string `json:"tool_name,omitempty"`
	CreatedAt int64  `json:"created_at"`
}

type PendingAction struct {
	ID        string         `json:"id"`
	ToolName  string         `json:"tool_name"`
	Arguments map[string]any `json:"arguments"`
	CreatedAt int64          `json:"created_at"`
	ExpireAt  int64          `json:"expire_at"`
}

type SessionState struct {
	LastUserWallet string         `json:"last_user_wallet,omitempty"`
	LastUserID     string         `json:"last_user_id,omitempty"`
	LastOrderID    string         `json:"last_order_id,omitempty"`
	PendingAction  *PendingAction `json:"pending_action,omitempty"`
}
