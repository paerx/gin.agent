package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gin.agent/pkg/audit"
	"gin.agent/pkg/auth"
	"gin.agent/pkg/ginai"
	"gin.agent/pkg/transport"
)

type Agent interface {
	HandleMessage(ctx context.Context, input AgentInput) (*AgentOutput, error)
}

type RoleResolver interface {
	Resolve(ctx context.Context, platform, userID string) ([]string, error)
}

type StaticRoleResolver map[string][]string

func (s StaticRoleResolver) Resolve(_ context.Context, _ string, userID string) ([]string, error) {
	return s[userID], nil
}

type Config struct {
	Registry           *ginai.Registry
	Memory             MemoryStore
	Planner            Planner
	Invoker            transport.Invoker
	PermissionChecker  auth.PermissionChecker
	Formatter          Formatter
	AuditStore         audit.Store
	RoleResolver       RoleResolver
	ConfirmTTL         time.Duration
	MaxContextMessages int
	MaxContextChars    int
}

type DefaultAgent struct {
	registry           *ginai.Registry
	memory             MemoryStore
	planner            Planner
	invoker            transport.Invoker
	permissionChecker  auth.PermissionChecker
	formatter          Formatter
	auditStore         audit.Store
	roleResolver       RoleResolver
	confirmTTL         time.Duration
	maxContextMessages int
	maxContextChars    int
}

func New(cfg Config) *DefaultAgent {
	ttl := cfg.ConfirmTTL
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	formatter := cfg.Formatter
	if formatter == nil {
		formatter = NewTextFormatter()
	}
	maxContextMessages := cfg.MaxContextMessages
	if maxContextMessages <= 0 {
		maxContextMessages = 28
	}
	maxContextChars := cfg.MaxContextChars
	if maxContextChars <= 0 {
		maxContextChars = 12000
	}
	return &DefaultAgent{
		registry:           cfg.Registry,
		memory:             cfg.Memory,
		planner:            cfg.Planner,
		invoker:            cfg.Invoker,
		permissionChecker:  cfg.PermissionChecker,
		formatter:          formatter,
		auditStore:         cfg.AuditStore,
		roleResolver:       cfg.RoleResolver,
		confirmTTL:         ttl,
		maxContextMessages: maxContextMessages,
		maxContextChars:    maxContextChars,
	}
}

func (a *DefaultAgent) HandleMessage(ctx context.Context, input AgentInput) (*AgentOutput, error) {
	conversationID := BuildConversationID(input.Platform, input.ChatType, input.ChatID, input.UserID)
	roles, _ := a.resolveRoles(ctx, input.Platform, input.UserID)

	if ok, err := a.memory.MarkMessageProcessed(ctx, input.Platform, input.MessageID); err != nil {
		return nil, err
	} else if !ok {
		return &AgentOutput{Text: "这条消息已经处理过了。"}, nil
	}

	userMessage := Message{Role: "user", Content: input.Text, CreatedAt: time.Now().Unix()}
	if err := a.memory.AppendMessage(ctx, conversationID, userMessage); err != nil {
		return nil, err
	}

	state, err := a.memory.GetState(ctx, conversationID)
	if err != nil {
		return nil, err
	}

	if result := a.handleBuiltinCommand(ctx, input, conversationID, roles); result.Handled {
		if result.Err != nil {
			return nil, result.Err
		}
		return &AgentOutput{Text: result.Text}, nil
	}

	trimmedText := strings.TrimSpace(input.Text)
	switch trimmedText {
	case "确认":
		return a.handleConfirm(ctx, input, conversationID, state, roles)
	case "取消":
		return a.handleCancel(ctx, conversationID, state)
	}

	messages, err := a.memory.GetMessages(ctx, conversationID, 30)
	if err != nil {
		return nil, err
	}
	if a.isContextTooLong(messages) {
		return &AgentOutput{Text: "你的上下文有点长了，为了避免理解偏差，请先发送 clean 清空上下文后再继续。"}, nil
	}

	plan, err := a.planner.Plan(ctx, PlannerInput{
		UserText:     input.Text,
		Messages:     messages,
		State:        state,
		Tools:        a.registry.ExportSchemas(),
		SystemPrompt: DefaultSystemPrompt,
	})
	if err != nil {
		return &AgentOutput{Text: "AI 服务暂时不可用，请稍后重试。"}, a.insertAudit(ctx, input, conversationID, audit.AuditLog{
			ID:             buildAuditID(input),
			Platform:       input.Platform,
			ChatID:         input.ChatID,
			UserID:         input.UserID,
			MessageID:      input.MessageID,
			ConversationID: conversationID,
			UserText:       input.Text,
			ErrorMessage:   err.Error(),
		})
	}

	switch plan.Type {
	case "text", "ask", "refuse":
		if err := a.memory.AppendMessage(ctx, conversationID, Message{Role: "assistant", Content: plan.Text, CreatedAt: time.Now().Unix()}); err != nil {
			return nil, err
		}
		return &AgentOutput{Text: plan.Text}, nil
	case "tool_call":
		return a.handleToolCall(ctx, input, conversationID, state, roles, plan.ToolCall)
	default:
		return &AgentOutput{Text: "我没理解你要执行哪个操作，可以再说具体一点吗？"}, nil
	}
}

func (a *DefaultAgent) isContextTooLong(messages []Message) bool {
	if len(messages) > a.maxContextMessages {
		return true
	}
	totalChars := 0
	for _, message := range messages {
		totalChars += len([]rune(message.Content))
	}
	return totalChars > a.maxContextChars
}

func (a *DefaultAgent) handleToolCall(ctx context.Context, input AgentInput, conversationID string, state SessionState, roles []string, call *ToolCall) (*AgentOutput, error) {
	tool, ok := a.registry.Get(call.ToolName)
	if !ok {
		return &AgentOutput{Text: "我没找到对应的工具，请确认操作名称。"}, nil
	}
	if err := applyStateHints(call.Arguments, &state); err != nil {
		return nil, err
	}
	if err := ginai.ValidateArguments(tool.Schema, call.Arguments); err != nil {
		_ = a.insertAudit(ctx, input, conversationID, audit.AuditLog{
			ID:             buildAuditID(input),
			Platform:       input.Platform,
			ChatID:         input.ChatID,
			UserID:         input.UserID,
			MessageID:      input.MessageID,
			ConversationID: conversationID,
			UserText:       input.Text,
			ToolName:       tool.Name,
			Arguments:      call.Arguments,
			NeedConfirm:    tool.NeedConfirm,
			RequestMethod:  tool.Method,
			RequestPath:    tool.Path,
			ErrorMessage:   err.Error(),
		})
		return &AgentOutput{Text: "参数格式不正确，请检查后重试。"}, nil
	}

	identity := auth.Identity{Platform: input.Platform, UserID: input.UserID, Roles: roles}
	if err := a.permissionChecker.CanCall(ctx, identity, tool); err != nil {
		_ = a.insertAudit(ctx, input, conversationID, audit.AuditLog{
			ID:             buildAuditID(input),
			Platform:       input.Platform,
			ChatID:         input.ChatID,
			UserID:         input.UserID,
			MessageID:      input.MessageID,
			ConversationID: conversationID,
			UserText:       input.Text,
			ToolName:       tool.Name,
			Arguments:      call.Arguments,
			NeedConfirm:    tool.NeedConfirm,
			PermissionPass: false,
			RequestMethod:  tool.Method,
			RequestPath:    tool.Path,
			ErrorMessage:   err.Error(),
		})
		return &AgentOutput{Text: "你没有权限执行这个操作。"}, nil
	}

	if !tool.ReadOnly && tool.NeedConfirm {
		pending := PendingAction{
			ID:        buildPendingID(input),
			ToolName:  tool.Name,
			Arguments: call.Arguments,
			CreatedAt: time.Now().Unix(),
			ExpireAt:  time.Now().Add(a.confirmTTL).Unix(),
		}
		state.PendingAction = &pending
		if err := a.memory.SetPendingAction(ctx, conversationID, pending); err != nil {
			return nil, err
		}
		if err := a.memory.SetState(ctx, conversationID, state); err != nil {
			return nil, err
		}
		text := fmt.Sprintf("将执行 %s，参数为 %s。回复“确认”执行，回复“取消”放弃。", tool.Name, summarizeArguments(call.Arguments))
		_ = a.insertAudit(ctx, input, conversationID, audit.AuditLog{
			ID:             buildAuditID(input),
			Platform:       input.Platform,
			ChatID:         input.ChatID,
			UserID:         input.UserID,
			MessageID:      input.MessageID,
			ConversationID: conversationID,
			UserText:       input.Text,
			ToolName:       tool.Name,
			Arguments:      call.Arguments,
			NeedConfirm:    true,
			PermissionPass: true,
			RequestMethod:  tool.Method,
			RequestPath:    tool.Path,
		})
		return &AgentOutput{Text: text}, nil
	}

	return a.executeTool(ctx, input, conversationID, tool, call.Arguments, true, false)
}

func (a *DefaultAgent) handleConfirm(ctx context.Context, input AgentInput, conversationID string, state SessionState, roles []string) (*AgentOutput, error) {
	pending, err := a.memory.GetPendingAction(ctx, conversationID)
	if err != nil {
		return nil, err
	}
	if pending == nil || state.PendingAction == nil {
		return &AgentOutput{Text: "当前没有待确认的操作。"}, nil
	}
	if pending.ExpireAt < time.Now().Unix() {
		_ = a.memory.ClearPendingAction(ctx, conversationID)
		state.PendingAction = nil
		_ = a.memory.SetState(ctx, conversationID, state)
		return &AgentOutput{Text: "上一次待确认操作已经过期，请重新发起。"}, nil
	}

	tool, ok := a.registry.Get(pending.ToolName)
	if !ok {
		return &AgentOutput{Text: "待确认工具不存在，请重新发起。"}, nil
	}

	identity := auth.Identity{Platform: input.Platform, UserID: input.UserID, Roles: roles}
	if err := a.permissionChecker.CanCall(ctx, identity, tool); err != nil {
		return &AgentOutput{Text: "你没有权限执行这个操作。"}, nil
	}

	return a.executeTool(ctx, input, conversationID, tool, pending.Arguments, true, true)
}

func (a *DefaultAgent) handleCancel(ctx context.Context, conversationID string, state SessionState) (*AgentOutput, error) {
	state.PendingAction = nil
	if err := a.memory.ClearPendingAction(ctx, conversationID); err != nil {
		return nil, err
	}
	if err := a.memory.SetState(context.Background(), conversationID, state); err != nil {
		return nil, err
	}
	return &AgentOutput{Text: "已取消待确认操作。"}, nil
}

func (a *DefaultAgent) executeTool(ctx context.Context, input AgentInput, conversationID string, tool *ginai.Tool, args map[string]any, permissionPass, confirmed bool) (*AgentOutput, error) {
	if err := validateToolArguments(tool, args); err != nil {
		return &AgentOutput{Text: "参数格式不正确，请检查后重试。"}, a.insertAudit(ctx, input, conversationID, audit.AuditLog{
			ID:             buildAuditID(input),
			Platform:       input.Platform,
			ChatID:         input.ChatID,
			UserID:         input.UserID,
			MessageID:      input.MessageID,
			ConversationID: conversationID,
			UserText:       input.Text,
			ToolName:       tool.Name,
			Arguments:      args,
			NeedConfirm:    tool.NeedConfirm,
			Confirmed:      confirmed,
			PermissionPass: permissionPass,
			RequestMethod:  tool.Method,
			RequestPath:    tool.Path,
			ErrorMessage:   err.Error(),
		})
	}
	result, err := a.invoker.Invoke(ctx, tool, args)
	if err != nil {
		_ = a.insertAudit(ctx, input, conversationID, audit.AuditLog{
			ID:             buildAuditID(input),
			Platform:       input.Platform,
			ChatID:         input.ChatID,
			UserID:         input.UserID,
			MessageID:      input.MessageID,
			ConversationID: conversationID,
			UserText:       input.Text,
			ToolName:       tool.Name,
			Arguments:      args,
			NeedConfirm:    tool.NeedConfirm,
			Confirmed:      confirmed,
			PermissionPass: permissionPass,
			RequestMethod:  tool.Method,
			RequestPath:    tool.Path,
			ErrorMessage:   err.Error(),
		})
		return &AgentOutput{Text: a.formatter.FormatError(ctx, err)}, nil
	}
	text := a.formatter.Format(ctx, tool, result)
	if err := a.memory.AppendMessage(ctx, conversationID, Message{Role: "assistant", Content: text, ToolName: tool.Name, CreatedAt: time.Now().Unix()}); err != nil {
		return nil, err
	}
	state, err := a.memory.GetState(ctx, conversationID)
	if err == nil {
		updateSessionStateFromArgs(&state, args)
		state.PendingAction = nil
		_ = a.memory.SetState(ctx, conversationID, state)
	}
	_ = a.memory.ClearPendingAction(ctx, conversationID)
	_ = a.insertAudit(ctx, input, conversationID, audit.AuditLog{
		ID:             buildAuditID(input),
		Platform:       input.Platform,
		ChatID:         input.ChatID,
		UserID:         input.UserID,
		MessageID:      input.MessageID,
		ConversationID: conversationID,
		UserText:       input.Text,
		ToolName:       tool.Name,
		Arguments:      args,
		NeedConfirm:    tool.NeedConfirm,
		Confirmed:      confirmed,
		PermissionPass: permissionPass,
		RequestMethod:  tool.Method,
		RequestPath:    tool.Path,
		ResponseStatus: result.StatusCode,
		ResponseBody:   string(result.RawBody),
	})
	return &AgentOutput{Text: text}, nil
}

func (a *DefaultAgent) resolveRoles(ctx context.Context, platform, userID string) ([]string, error) {
	if a.roleResolver == nil {
		return []string{"readonly"}, nil
	}
	roles, err := a.roleResolver.Resolve(ctx, platform, userID)
	if err != nil {
		return nil, err
	}
	if len(roles) == 0 {
		return []string{"readonly"}, nil
	}
	return roles, nil
}

func (a *DefaultAgent) insertAudit(ctx context.Context, input AgentInput, conversationID string, log audit.AuditLog) error {
	if a.auditStore == nil {
		return nil
	}
	if log.ID == "" {
		log.ID = buildAuditID(input)
	}
	log.Platform = defaultIfEmpty(log.Platform, input.Platform)
	log.ChatID = defaultIfEmpty(log.ChatID, input.ChatID)
	log.UserID = defaultIfEmpty(log.UserID, input.UserID)
	log.MessageID = defaultIfEmpty(log.MessageID, input.MessageID)
	log.ConversationID = defaultIfEmpty(log.ConversationID, conversationID)
	return a.auditStore.Insert(ctx, log)
}

func applyStateHints(args map[string]any, state *SessionState) error {
	if args == nil {
		return nil
	}
	if isBlankValue(args["wallet"]) && state.LastUserWallet != "" {
		args["wallet"] = state.LastUserWallet
	}
	if isBlankValue(args["user_id"]) && state.LastUserID != "" {
		args["user_id"] = state.LastUserID
	}
	if isBlankValue(args["order_id"]) && state.LastOrderID != "" {
		args["order_id"] = state.LastOrderID
	}
	return nil
}

func updateSessionStateFromArgs(state *SessionState, args map[string]any) {
	if v, ok := args["wallet"].(string); ok && strings.TrimSpace(v) != "" {
		state.LastUserWallet = v
	}
	if v, ok := args["user_id"].(string); ok && strings.TrimSpace(v) != "" {
		state.LastUserID = v
	}
	if v, ok := args["order_id"].(string); ok && strings.TrimSpace(v) != "" {
		state.LastOrderID = v
	}
}

func validateToolArguments(tool *ginai.Tool, args map[string]any) error {
	if err := ginai.ValidateArguments(tool.Schema, args); err != nil {
		return err
	}
	if tool.ReadOnly || len(tool.AllowFields) == 0 {
		return nil
	}
	field, ok := args["field"].(string)
	if !ok || field == "" {
		return nil
	}
	for _, allowed := range tool.AllowFields {
		if allowed == field {
			return nil
		}
	}
	return fmt.Errorf("field %s is not allowed", field)
}

func summarizeArguments(args map[string]any) string {
	raw, _ := json.Marshal(audit.MaskSensitiveMap(args))
	return string(raw)
}

func buildAuditID(input AgentInput) string {
	return fmt.Sprintf("%s:%s:%d", input.Platform, input.MessageID, time.Now().UnixNano())
}

func buildPendingID(input AgentInput) string {
	return fmt.Sprintf("%s:%s", input.UserID, input.MessageID)
}

func defaultIfEmpty(current, fallback string) string {
	if current == "" {
		return fallback
	}
	return current
}

func isBlankValue(v any) bool {
	switch typed := v.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(typed) == ""
	default:
		return false
	}
}
