package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/paerx/gin.agent/pkg/agent"

	"github.com/redis/go-redis/v9"
)

type RedisMemoryConfig struct {
	Client     *redis.Client
	TTL        time.Duration
	MaxHistory int64
}

type RedisMemoryStore struct {
	client     *redis.Client
	ttl        time.Duration
	maxHistory int64
}

func NewRedisMemoryStore(cfg RedisMemoryConfig) *RedisMemoryStore {
	ttl := cfg.TTL
	if ttl <= 0 {
		ttl = 7 * 24 * time.Hour
	}
	maxHistory := cfg.MaxHistory
	if maxHistory <= 0 {
		maxHistory = 30
	}
	return &RedisMemoryStore{
		client:     cfg.Client,
		ttl:        ttl,
		maxHistory: maxHistory,
	}
}

func (s *RedisMemoryStore) AppendMessage(ctx context.Context, conversationID string, message agent.Message) error {
	raw, err := json.Marshal(message)
	if err != nil {
		return err
	}
	key := s.messagesKey(conversationID)
	pipe := s.client.TxPipeline()
	pipe.LPush(ctx, key, raw)
	pipe.LTrim(ctx, key, 0, s.maxHistory-1)
	pipe.Expire(ctx, key, s.ttl)
	_, err = pipe.Exec(ctx)
	return err
}

func (s *RedisMemoryStore) GetMessages(ctx context.Context, conversationID string, limit int64) ([]agent.Message, error) {
	if limit <= 0 {
		limit = s.maxHistory
	}
	items, err := s.client.LRange(ctx, s.messagesKey(conversationID), 0, limit-1).Result()
	if err != nil {
		return nil, err
	}
	out := make([]agent.Message, 0, len(items))
	for i := len(items) - 1; i >= 0; i-- {
		var message agent.Message
		if err := json.Unmarshal([]byte(items[i]), &message); err != nil {
			return nil, err
		}
		out = append(out, message)
	}
	return out, nil
}

func (s *RedisMemoryStore) SetState(ctx context.Context, conversationID string, state agent.SessionState) error {
	raw, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return s.client.Set(ctx, s.stateKey(conversationID), raw, s.ttl).Err()
}

func (s *RedisMemoryStore) GetState(ctx context.Context, conversationID string) (agent.SessionState, error) {
	raw, err := s.client.Get(ctx, s.stateKey(conversationID)).Bytes()
	if errors.Is(err, redis.Nil) {
		return agent.SessionState{}, nil
	}
	if err != nil {
		return agent.SessionState{}, err
	}
	var state agent.SessionState
	return state, json.Unmarshal(raw, &state)
}

func (s *RedisMemoryStore) SetPendingAction(ctx context.Context, conversationID string, action agent.PendingAction) error {
	raw, err := json.Marshal(action)
	if err != nil {
		return err
	}
	return s.client.Set(ctx, s.pendingKey(conversationID), raw, s.ttl).Err()
}

func (s *RedisMemoryStore) GetPendingAction(ctx context.Context, conversationID string) (*agent.PendingAction, error) {
	raw, err := s.client.Get(ctx, s.pendingKey(conversationID)).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var pending agent.PendingAction
	if err := json.Unmarshal(raw, &pending); err != nil {
		return nil, err
	}
	return &pending, nil
}

func (s *RedisMemoryStore) ClearPendingAction(ctx context.Context, conversationID string) error {
	return s.client.Del(ctx, s.pendingKey(conversationID)).Err()
}

func (s *RedisMemoryStore) ClearConversation(ctx context.Context, conversationID string) error {
	return s.client.Del(ctx, s.messagesKey(conversationID), s.stateKey(conversationID), s.pendingKey(conversationID)).Err()
}

func (s *RedisMemoryStore) MarkMessageProcessed(ctx context.Context, platform, messageID string) (bool, error) {
	ok, err := s.client.SetNX(ctx, fmt.Sprintf("ai:%s:msg:%s", platform, messageID), "1", 24*time.Hour).Result()
	return ok, err
}

func (s *RedisMemoryStore) messagesKey(conversationID string) string {
	return "ai:messages:" + conversationID
}

func (s *RedisMemoryStore) stateKey(conversationID string) string {
	return "ai:state:" + conversationID
}

func (s *RedisMemoryStore) pendingKey(conversationID string) string {
	return "ai:pending:" + conversationID
}
