package auth

import (
	"context"
	"testing"

	"gin.agent/pkg/ginai"
)

func TestPermissionChecker(t *testing.T) {
	checker := NewStaticChecker()
	readonlyTool := &ginai.Tool{Name: "get_user", Method: "GET", ReadOnly: true}
	writeTool := &ginai.Tool{Name: "update_user", Method: "POST", ReadOnly: false, Roles: []string{"operator"}}
	dangerTool := &ginai.Tool{Name: "danger", Method: "POST", Dangerous: true, Roles: []string{"operator"}}

	if err := checker.CanCall(context.Background(), Identity{Roles: []string{"readonly"}}, readonlyTool); err != nil {
		t.Fatalf("readonly tool should pass: %v", err)
	}
	if err := checker.CanCall(context.Background(), Identity{Roles: []string{"readonly"}}, writeTool); err == nil {
		t.Fatal("readonly user should not call write tool")
	}
	if err := checker.CanCall(context.Background(), Identity{Roles: []string{"operator"}}, dangerTool); err == nil {
		t.Fatal("non-admin should not call dangerous tool")
	}
}
