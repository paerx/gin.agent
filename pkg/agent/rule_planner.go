package agent

import (
	"context"
	"strings"
)

// RulePlanner is a tiny fallback planner used by tests and local demos.
type RulePlanner struct{}

func NewRulePlanner() *RulePlanner {
	return &RulePlanner{}
}

func (p *RulePlanner) Plan(_ context.Context, input PlannerInput) (*PlanResult, error) {
	text := strings.TrimSpace(input.UserText)
	switch {
	case strings.Contains(text, "查") && strings.Contains(text, "用户"):
		wallet := findTokenWithPrefix(text, "0x")
		return &PlanResult{
			Type: "tool_call",
			ToolCall: &ToolCall{
				ToolName: "get_user_info",
				Arguments: map[string]any{
					"wallet": wallet,
				},
			},
		}, nil
	case strings.Contains(text, "昵称") && strings.Contains(text, "改成"):
		value := text[strings.Index(text, "改成")+len("改成"):]
		value = strings.TrimSpace(value)
		return &PlanResult{
			Type: "tool_call",
			ToolCall: &ToolCall{
				ToolName: "update_user_info",
				Arguments: map[string]any{
					"wallet": "",
					"field":  "nickname",
					"value":  value,
				},
			},
		}, nil
	default:
		return &PlanResult{Type: "ask", Text: "我没理解你要执行哪个操作，可以再说具体一点吗？"}, nil
	}
}

func findTokenWithPrefix(text, prefix string) string {
	for _, part := range strings.Fields(text) {
		if strings.HasPrefix(strings.ToLower(part), strings.ToLower(prefix)) {
			return part
		}
	}
	return ""
}
