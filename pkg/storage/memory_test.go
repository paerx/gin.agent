package storage

import (
	"context"
	"testing"
	"time"

	"gin.agent/pkg/agent"
)

func TestMemoryStore(t *testing.T) {
	store := NewMemoryStore(time.Hour, 2)
	ctx := context.Background()
	conversationID := agent.BuildConversationID("lark", "group", "chat1", "user1")

	_ = store.AppendMessage(ctx, conversationID, agent.Message{Role: "user", Content: "1"})
	_ = store.AppendMessage(ctx, conversationID, agent.Message{Role: "assistant", Content: "2"})
	_ = store.AppendMessage(ctx, conversationID, agent.Message{Role: "user", Content: "3"})

	messages, err := store.GetMessages(ctx, conversationID, 30)
	if err != nil {
		t.Fatalf("GetMessages() error = %v", err)
	}
	if len(messages) != 2 || messages[0].Content != "2" || messages[1].Content != "3" {
		t.Fatalf("unexpected messages = %#v", messages)
	}

	ok, err := store.MarkMessageProcessed(ctx, "lark", "msg1")
	if err != nil {
		t.Fatalf("MarkMessageProcessed() error = %v", err)
	}
	if !ok {
		t.Fatal("expected first message processing to pass")
	}
	ok, err = store.MarkMessageProcessed(ctx, "lark", "msg1")
	if err != nil {
		t.Fatalf("MarkMessageProcessed() second error = %v", err)
	}
	if ok {
		t.Fatal("expected duplicate message to be rejected")
	}
}
