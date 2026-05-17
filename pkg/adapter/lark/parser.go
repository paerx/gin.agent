package lark

import (
	"encoding/json"
	"fmt"
	"strings"

	"gin.agent/pkg/agent"
)

func ParseAgentInput(req EventRequest) (*agent.AgentInput, bool, error) {
	if req.Event.Message.MessageType != "text" {
		return nil, false, nil
	}
	if req.Event.Sender.SenderType == "bot" {
		return nil, true, nil
	}
	var content textContent
	if err := json.Unmarshal([]byte(req.Event.Message.Content), &content); err != nil {
		return nil, false, fmt.Errorf("parse lark text content: %w", err)
	}
	userID := req.Event.Sender.SenderID.UserID
	if userID == "" {
		userID = req.Event.Sender.SenderID.OpenID
	}
	return &agent.AgentInput{
		Platform:    "lark",
		ChatID:      req.Event.Message.ChatID,
		ChatType:    normalizeChatType(req.Event.Message.ChatType),
		UserID:      userID,
		DisplayName: userID,
		MessageID:   req.Event.Message.MessageID,
		Text:        strings.TrimSpace(content.Text),
	}, false, nil
}

func normalizeChatType(chatType string) string {
	if chatType == "p2p" {
		return "p2p"
	}
	return "group"
}
