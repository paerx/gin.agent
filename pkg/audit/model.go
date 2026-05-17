package audit

import "time"

type AuditLog struct {
	ID             string         `json:"id"`
	Platform       string         `json:"platform"`
	ChatID         string         `json:"chat_id"`
	UserID         string         `json:"user_id"`
	MessageID      string         `json:"message_id"`
	ConversationID string         `json:"conversation_id"`
	UserText       string         `json:"user_text"`
	ToolName       string         `json:"tool_name"`
	Arguments      map[string]any `json:"arguments"`
	NeedConfirm    bool           `json:"need_confirm"`
	Confirmed      bool           `json:"confirmed"`
	PermissionPass bool           `json:"permission_pass"`
	RequestMethod  string         `json:"request_method"`
	RequestPath    string         `json:"request_path"`
	ResponseStatus int            `json:"response_status"`
	ResponseBody   string         `json:"response_body"`
	ErrorMessage   string         `json:"error_message"`
	CreatedAt      time.Time      `json:"created_at"`
}
