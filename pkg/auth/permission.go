package auth

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/paerx/gin.agent/pkg/ginai"
)

var (
	ErrPermissionDenied = errors.New("permission denied")
	ErrDeleteForbidden  = errors.New("delete is forbidden")
)

type Identity struct {
	Platform string   `json:"platform"`
	UserID   string   `json:"user_id"`
	Roles    []string `json:"roles"`
}

type PermissionChecker interface {
	CanCall(ctx context.Context, user Identity, tool *ginai.Tool) error
}

type StaticChecker struct{}

func NewStaticChecker() *StaticChecker {
	return &StaticChecker{}
}

func (c *StaticChecker) CanCall(_ context.Context, user Identity, tool *ginai.Tool) error {
	if strings.EqualFold(tool.Method, "DELETE") {
		return ErrDeleteForbidden
	}
	if tool.Dangerous && !slices.Contains(user.Roles, "admin") {
		return fmt.Errorf("%w: admin role required", ErrPermissionDenied)
	}
	if len(tool.Roles) == 0 {
		if tool.ReadOnly && slices.Contains(user.Roles, "readonly") {
			return nil
		}
		if tool.ReadOnly && len(user.Roles) == 0 {
			return nil
		}
		return fmt.Errorf("%w: tool roles are not configured", ErrPermissionDenied)
	}
	for _, role := range tool.Roles {
		if slices.Contains(user.Roles, role) {
			return nil
		}
	}
	return fmt.Errorf("%w: missing required role", ErrPermissionDenied)
}
