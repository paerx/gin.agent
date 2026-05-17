package agent

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"
)

type RoleStore interface {
	RoleResolver
	SetRoles(ctx context.Context, platform, userID string, roles []string) error
	RemoveUser(ctx context.Context, platform, userID string) error
	CreateRoleRequest(ctx context.Context, request RoleRequest) (RoleRequest, error)
	GetRoleRequest(ctx context.Context, id string) (RoleRequest, bool, error)
	DeleteRoleRequest(ctx context.Context, id string) error
	ListRoleRequests(ctx context.Context) ([]RoleRequest, error)
}

type RoleRequest struct {
	ID          string   `json:"id"`
	Platform    string   `json:"platform"`
	ChatID      string   `json:"chat_id"`
	UserID      string   `json:"user_id"`
	DisplayName string   `json:"display_name,omitempty"`
	Roles       []string `json:"roles"`
	CreatedAt   int64    `json:"created_at"`
}

type MemoryRoleStore struct {
	mu       sync.RWMutex
	roles    map[string][]string
	requests map[string]RoleRequest
}

func NewMemoryRoleStore(initial map[string][]string) *MemoryRoleStore {
	store := &MemoryRoleStore{
		roles:    make(map[string][]string),
		requests: make(map[string]RoleRequest),
	}
	for userID, roles := range initial {
		store.roles[userID] = normalizeRoles(roles)
	}
	return store
}

func (s *MemoryRoleStore) Resolve(_ context.Context, _ string, userID string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return slices.Clone(s.roles[userID]), nil
}

func (s *MemoryRoleStore) SetRoles(_ context.Context, _ string, userID string, roles []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.roles[userID] = normalizeRoles(roles)
	return nil
}

func (s *MemoryRoleStore) RemoveUser(_ context.Context, _ string, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.roles, userID)
	return nil
}

func (s *MemoryRoleStore) CreateRoleRequest(_ context.Context, request RoleRequest) (RoleRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	request.Roles = normalizeRoles(request.Roles)
	if request.CreatedAt == 0 {
		request.CreatedAt = time.Now().Unix()
	}
	if request.ID == "" {
		request.ID = fmt.Sprintf("req-%d", time.Now().UnixNano())
	}
	s.requests[request.ID] = request
	return request, nil
}

func (s *MemoryRoleStore) GetRoleRequest(_ context.Context, id string) (RoleRequest, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	request, ok := s.requests[id]
	request.Roles = slices.Clone(request.Roles)
	return request, ok, nil
}

func (s *MemoryRoleStore) DeleteRoleRequest(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.requests, id)
	return nil
}

func (s *MemoryRoleStore) ListRoleRequests(_ context.Context) ([]RoleRequest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]RoleRequest, 0, len(s.requests))
	for _, request := range s.requests {
		request.Roles = slices.Clone(request.Roles)
		out = append(out, request)
	}
	return out, nil
}

func normalizeRoles(roles []string) []string {
	out := make([]string, 0, len(roles))
	for _, role := range roles {
		if role == "" || slices.Contains(out, role) {
			continue
		}
		out = append(out, role)
	}
	return out
}
