package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"gin.agent/pkg/audit"
	"gin.agent/pkg/ginai"
	"gin.agent/pkg/transport"
)

type Formatter interface {
	Format(ctx context.Context, tool *ginai.Tool, result *transport.InvokeResult) string
	FormatError(ctx context.Context, err error) string
}

type TextFormatter struct{}

func NewTextFormatter() *TextFormatter {
	return &TextFormatter{}
}

func (f *TextFormatter) Format(_ context.Context, tool *ginai.Tool, result *transport.InvokeResult) string {
	var payload any
	if err := json.Unmarshal(result.RawBody, &payload); err != nil {
		return result.Summary
	}
	switch typed := payload.(type) {
	case map[string]any:
		masked := audit.MaskSensitiveMap(typed)
		lines := []string{"查询结果："}
		keys := make([]string, 0, len(masked))
		for key := range masked {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			lines = append(lines, fmt.Sprintf("%s：%v", key, masked[key]))
		}
		return strings.Join(lines, "\n")
	case []any:
		return fmt.Sprintf("%s 返回 %d 条记录。", tool.Name, len(typed))
	default:
		return result.Summary
	}
}

func (f *TextFormatter) FormatError(_ context.Context, err error) string {
	return "调用接口失败，已记录日志。"
}
