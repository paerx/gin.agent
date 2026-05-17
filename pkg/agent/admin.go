package agent

import (
	"context"
	"fmt"
	"slices"
	"strings"
)

type adminCommandResult struct {
	Handled bool
	Text    string
	Err     error
}

func (a *DefaultAgent) handleBuiltinCommand(ctx context.Context, input AgentInput, conversationID string, roles []string) adminCommandResult {
	text := strings.TrimSpace(strings.TrimPrefix(input.Text, "@gin.agent"))
	if strings.EqualFold(text, "clean") || strings.EqualFold(text, "/clean") {
		if err := a.memory.ClearConversation(ctx, conversationID); err != nil {
			return adminCommandResult{Handled: true, Err: err}
		}
		return adminCommandResult{Handled: true, Text: "已清空你的上下文。"}
	}
	if strings.EqualFold(text, "myuserid") || strings.EqualFold(text, "/myuserid") {
		displayName := defaultIfEmpty(input.DisplayName, input.UserID)
		return adminCommandResult{
			Handled: true,
			Text:    fmt.Sprintf("你的 UserID：%s\n显示名：%s", input.UserID, displayName),
		}
	}

	roleStore, ok := a.roleResolver.(RoleStore)
	if !ok {
		return adminCommandResult{}
	}

	fields := strings.Fields(text)
	if len(fields) == 0 {
		return adminCommandResult{}
	}

	if strings.EqualFold(fields[0], "addme") {
		if len(fields) < 2 {
			return adminCommandResult{Handled: true, Text: "申请权限格式：addme <role...>，例如 addme operator readonly。"}
		}
		request, err := roleStore.CreateRoleRequest(ctx, RoleRequest{
			Platform:    input.Platform,
			ChatID:      input.ChatID,
			UserID:      input.UserID,
			DisplayName: defaultIfEmpty(input.DisplayName, input.UserID),
			Roles:       fields[1:],
		})
		if err != nil {
			return adminCommandResult{Handled: true, Err: err}
		}
		return adminCommandResult{
			Handled: true,
			Text: fmt.Sprintf(
				"已提交权限申请 %s。\n申请人：%s\nUserID：%s\n申请权限：%s\nOwner 可回复 approve %s 批准，或 reject %s 拒绝。",
				request.ID,
				defaultIfEmpty(request.DisplayName, request.UserID),
				request.UserID,
				strings.Join(request.Roles, ", "),
				request.ID,
				request.ID,
			),
		}
	}

	if !isOwner(roles) {
		if isRoleAdminCommand(fields[0]) {
			return adminCommandResult{Handled: true, Text: "只有 owner/admin 可以管理权限。你可以发送 addme <role...> 申请权限。"}
		}
		return adminCommandResult{}
	}

	switch strings.ToLower(fields[0]) {
	case "add":
		if len(fields) >= 4 && strings.EqualFold(fields[1], "user") {
			userID := fields[2]
			newRoles := fields[3:]
			if err := roleStore.SetRoles(ctx, input.Platform, userID, newRoles); err != nil {
				return adminCommandResult{Handled: true, Err: err}
			}
			return adminCommandResult{Handled: true, Text: fmt.Sprintf("已为用户 %s 设置权限：%s。", userID, strings.Join(newRoles, ", "))}
		}
	case "remove":
		if len(fields) == 3 && strings.EqualFold(fields[1], "user") {
			userID := fields[2]
			if err := roleStore.RemoveUser(ctx, input.Platform, userID); err != nil {
				return adminCommandResult{Handled: true, Err: err}
			}
			return adminCommandResult{Handled: true, Text: fmt.Sprintf("已移除用户 %s 的权限。", userID)}
		}
	case "roles":
		if len(fields) == 2 {
			userID := fields[1]
			userRoles, err := roleStore.Resolve(ctx, input.Platform, userID)
			if err != nil {
				return adminCommandResult{Handled: true, Err: err}
			}
			if len(userRoles) == 0 {
				return adminCommandResult{Handled: true, Text: fmt.Sprintf("用户 %s 当前没有配置权限。", userID)}
			}
			return adminCommandResult{Handled: true, Text: fmt.Sprintf("用户 %s 的权限：%s。", userID, strings.Join(userRoles, ", "))}
		}
	case "approve":
		if len(fields) == 2 {
			requestID := fields[1]
			request, ok, err := roleStore.GetRoleRequest(ctx, requestID)
			if err != nil {
				return adminCommandResult{Handled: true, Err: err}
			}
			if !ok {
				return adminCommandResult{Handled: true, Text: fmt.Sprintf("没有找到权限申请 %s。", requestID)}
			}
			if err := roleStore.SetRoles(ctx, request.Platform, request.UserID, request.Roles); err != nil {
				return adminCommandResult{Handled: true, Err: err}
			}
			_ = roleStore.DeleteRoleRequest(ctx, requestID)
			return adminCommandResult{
				Handled: true,
				Text:    fmt.Sprintf("已批准 %s 的权限申请：%s。", defaultIfEmpty(request.DisplayName, request.UserID), strings.Join(request.Roles, ", ")),
			}
		}
	case "reject":
		if len(fields) == 2 {
			requestID := fields[1]
			if err := roleStore.DeleteRoleRequest(ctx, requestID); err != nil {
				return adminCommandResult{Handled: true, Err: err}
			}
			return adminCommandResult{Handled: true, Text: fmt.Sprintf("已拒绝权限申请 %s。", requestID)}
		}
	case "requests":
		requests, err := roleStore.ListRoleRequests(ctx)
		if err != nil {
			return adminCommandResult{Handled: true, Err: err}
		}
		if len(requests) == 0 {
			return adminCommandResult{Handled: true, Text: "当前没有待审批的权限申请。"}
		}
		lines := []string{"待审批权限申请："}
		for _, request := range requests {
			lines = append(lines, fmt.Sprintf("%s：%s (%s) -> %s", request.ID, defaultIfEmpty(request.DisplayName, request.UserID), request.UserID, strings.Join(request.Roles, ", ")))
		}
		return adminCommandResult{Handled: true, Text: strings.Join(lines, "\n")}
	}

	lowerText := strings.ToLower(text)
	if strings.HasPrefix(lowerText, "add user") ||
		strings.HasPrefix(lowerText, "remove user") ||
		strings.HasPrefix(lowerText, "roles") ||
		strings.HasPrefix(lowerText, "approve") ||
		strings.HasPrefix(lowerText, "reject") {
		return adminCommandResult{
			Handled: true,
			Text:    "权限指令格式：add user <user_id> <role...>，remove user <user_id>，roles <user_id>，approve <request_id>，reject <request_id>，requests。",
		}
	}

	return adminCommandResult{}
}

func isOwner(roles []string) bool {
	return slices.Contains(roles, "owner") || slices.Contains(roles, "admin")
}

func isRoleAdminCommand(command string) bool {
	switch strings.ToLower(command) {
	case "add", "remove", "roles", "approve", "reject", "requests":
		return true
	default:
		return false
	}
}
