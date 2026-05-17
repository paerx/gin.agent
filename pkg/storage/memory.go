package storage

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/paerx/gin.agent/pkg/agent"
)

type MemoryStore struct {
	mu          sync.Mutex
	ttl         time.Duration
	maxHistory  int64
	messages    map[string][]agent.Message
	state       map[string]agent.SessionState
	pending     map[string]agent.PendingAction
	processed   map[string]time.Time
	stateExpiry map[string]time.Time
}

func NewMemoryStore(ttl time.Duration, maxHistory int64) *MemoryStore {
	if ttl <= 0 {
		ttl = 7 * 24 * time.Hour
	}
	if maxHistory <= 0 {
		maxHistory = 30
	}
	return &MemoryStore{
		ttl:         ttl,
		maxHistory:  maxHistory,
		messages:    make(map[string][]agent.Message),
		state:       make(map[string]agent.SessionState),
		pending:     make(map[string]agent.PendingAction),
		processed:   make(map[string]time.Time),
		stateExpiry: make(map[string]time.Time),
	}
}

func (s *MemoryStore) AppendMessage(_ context.Context, conversationID string, message agent.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupLocked(time.Now())
	items := append(s.messages[conversationID], message)
	if int64(len(items)) > s.maxHistory {
		items = items[len(items)-int(s.maxHistory):]
	}
	s.messages[conversationID] = items
	s.stateExpiry[conversationID] = time.Now().Add(s.ttl)
	return nil
}

func (s *MemoryStore) GetMessages(_ context.Context, conversationID string, limit int64) ([]agent.Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupLocked(time.Now())
	items := s.messages[conversationID]
	if limit <= 0 || limit > int64(len(items)) {
		limit = int64(len(items))
	}
	return slices.Clone(items[len(items)-int(limit):]), nil
}

func (s *MemoryStore) SetState(_ context.Context, conversationID string, state agent.SessionState) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state[conversationID] = state
	s.stateExpiry[conversationID] = time.Now().Add(s.ttl)
	return nil
}

func (s *MemoryStore) GetState(_ context.Context, conversationID string) (agent.SessionState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupLocked(time.Now())
	return s.state[conversationID], nil
}

func (s *MemoryStore) SetPendingAction(_ context.Context, conversationID string, action agent.PendingAction) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pending[conversationID] = action
	s.stateExpiry[conversationID] = time.Now().Add(s.ttl)
	return nil
}

func (s *MemoryStore) GetPendingAction(_ context.Context, conversationID string) (*agent.PendingAction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupLocked(time.Now())
	pending, ok := s.pending[conversationID]
	if !ok {
		return nil, nil
	}
	return &pending, nil
}

func (s *MemoryStore) ClearPendingAction(_ context.Context, conversationID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.pending, conversationID)
	return nil
}

func (s *MemoryStore) ClearConversation(_ context.Context, conversationID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.messages, conversationID)
	delete(s.state, conversationID)
	delete(s.pending, conversationID)
	delete(s.stateExpiry, conversationID)
	return nil
}

func (s *MemoryStore) MarkMessageProcessed(_ context.Context, platform, messageID string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupLocked(time.Now())
	key := "ai:" + platform + ":msg:" + messageID
	if _, ok := s.processed[key]; ok {
		return false, nil
	}
	s.processed[key] = time.Now().Add(24 * time.Hour)
	return true, nil
}

func (s *MemoryStore) cleanupLocked(now time.Time) {
	for key, expiresAt := range s.stateExpiry {
		if now.After(expiresAt) {
			delete(s.messages, key)
			delete(s.state, key)
			delete(s.pending, key)
			delete(s.stateExpiry, key)
		}
	}
	for key, expiresAt := range s.processed {
		if now.After(expiresAt) {
			delete(s.processed, key)
		}
	}
}
