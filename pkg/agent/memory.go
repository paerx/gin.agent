package agent

import (
	"context"
	"fmt"
)

type MemoryStore interface {
	AppendMessage(ctx context.Context, conversationID string, message Message) error
	GetMessages(ctx context.Context, conversationID string, limit int64) ([]Message, error)
	SetState(ctx context.Context, conversationID string, state SessionState) error
	GetState(ctx context.Context, conversationID string) (SessionState, error)
	SetPendingAction(ctx context.Context, conversationID string, action PendingAction) error
	GetPendingAction(ctx context.Context, conversationID string) (*PendingAction, error)
	ClearPendingAction(ctx context.Context, conversationID string) error
	ClearConversation(ctx context.Context, conversationID string) error
	MarkMessageProcessed(ctx context.Context, platform, messageID string) (bool, error)
}

func BuildConversationID(platform, chatType, chatID, userID string) string {
	if chatType == "p2p" {
		return platform + ":private:" + userID
	}
	return fmt.Sprintf("%s:group:%s:user:%s", platform, chatID, userID)
}
