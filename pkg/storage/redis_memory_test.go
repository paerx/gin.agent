package storage

import (
	"context"
	"testing"
	"time"

	"gin.agent/pkg/agent"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestRedisMemoryStore(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() error = %v", err)
	}
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	store := NewRedisMemoryStore(RedisMemoryConfig{
		Client:     client,
		TTL:        time.Hour,
		MaxHistory: 2,
	})
	ctx := context.Background()
	conversationID := agent.BuildConversationID("lark", "p2p", "chat", "user")

	_ = store.AppendMessage(ctx, conversationID, agent.Message{Role: "user", Content: "1"})
	_ = store.AppendMessage(ctx, conversationID, agent.Message{Role: "assistant", Content: "2"})
	_ = store.AppendMessage(ctx, conversationID, agent.Message{Role: "user", Content: "3"})

	messages, err := store.GetMessages(ctx, conversationID, 10)
	if err != nil {
		t.Fatalf("GetMessages() error = %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("message count = %d", len(messages))
	}

	state := agent.SessionState{LastUserWallet: "0xabc"}
	if err := store.SetState(ctx, conversationID, state); err != nil {
		t.Fatalf("SetState() error = %v", err)
	}
	gotState, err := store.GetState(ctx, conversationID)
	if err != nil {
		t.Fatalf("GetState() error = %v", err)
	}
	if gotState.LastUserWallet != "0xabc" {
		t.Fatalf("LastUserWallet = %s", gotState.LastUserWallet)
	}

	pending := agent.PendingAction{ID: "1", ToolName: "update_user", ExpireAt: time.Now().Add(time.Minute).Unix()}
	if err := store.SetPendingAction(ctx, conversationID, pending); err != nil {
		t.Fatalf("SetPendingAction() error = %v", err)
	}
	gotPending, err := store.GetPendingAction(ctx, conversationID)
	if err != nil {
		t.Fatalf("GetPendingAction() error = %v", err)
	}
	if gotPending == nil || gotPending.ToolName != "update_user" {
		t.Fatalf("pending action = %#v", gotPending)
	}
}
