package audit

import "context"

type Store interface {
	Insert(ctx context.Context, log AuditLog) error
}
