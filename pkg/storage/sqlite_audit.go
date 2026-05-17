package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"gin.agent/pkg/audit"

	_ "modernc.org/sqlite"
)

type SQLiteAuditStore struct {
	db *sql.DB
}

func NewSQLiteAuditStore(dsn string) (*SQLiteAuditStore, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	store := &SQLiteAuditStore{db: db}
	if err := store.init(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *SQLiteAuditStore) Insert(ctx context.Context, log audit.AuditLog) error {
	args := audit.MaskSensitiveMap(log.Arguments)
	rawArgs, err := json.Marshal(args)
	if err != nil {
		return err
	}
	if log.CreatedAt.IsZero() {
		log.CreatedAt = time.Now()
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO ai_audit_logs (
			id, platform, chat_id, user_id, message_id, conversation_id,
			user_text, tool_name, arguments, need_confirm, confirmed, permission_pass,
			request_method, request_path, response_status, response_body, error_message, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, log.ID, log.Platform, log.ChatID, log.UserID, log.MessageID, log.ConversationID,
		log.UserText, log.ToolName, string(rawArgs), log.NeedConfirm, log.Confirmed, log.PermissionPass,
		log.RequestMethod, log.RequestPath, log.ResponseStatus, log.ResponseBody, log.ErrorMessage, log.CreatedAt)
	return err
}

func (s *SQLiteAuditStore) Count(ctx context.Context) (int, error) {
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM ai_audit_logs`).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (s *SQLiteAuditStore) Latest(ctx context.Context) (*audit.AuditLog, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, platform, chat_id, user_id, message_id, conversation_id,
			user_text, tool_name, arguments, need_confirm, confirmed, permission_pass,
			request_method, request_path, response_status, response_body, error_message, created_at
		FROM ai_audit_logs
		ORDER BY created_at DESC, id DESC
		LIMIT 1
	`)
	return scanAuditLog(row)
}

type scanner interface {
	Scan(dest ...any) error
}

func scanAuditLog(row scanner) (*audit.AuditLog, error) {
	var log audit.AuditLog
	var rawArgs string
	if err := row.Scan(
		&log.ID, &log.Platform, &log.ChatID, &log.UserID, &log.MessageID, &log.ConversationID,
		&log.UserText, &log.ToolName, &rawArgs, &log.NeedConfirm, &log.Confirmed, &log.PermissionPass,
		&log.RequestMethod, &log.RequestPath, &log.ResponseStatus, &log.ResponseBody, &log.ErrorMessage, &log.CreatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if rawArgs != "" {
		if err := json.Unmarshal([]byte(rawArgs), &log.Arguments); err != nil {
			return nil, fmt.Errorf("decode audit arguments: %w", err)
		}
	}
	return &log, nil
}

func (s *SQLiteAuditStore) init() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS ai_audit_logs (
    id TEXT PRIMARY KEY,
    platform TEXT NOT NULL,
    chat_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    message_id TEXT,
    conversation_id TEXT NOT NULL,
    user_text TEXT,
    tool_name TEXT,
    arguments TEXT,
    need_confirm BOOLEAN DEFAULT FALSE,
    confirmed BOOLEAN DEFAULT FALSE,
    permission_pass BOOLEAN DEFAULT FALSE,
    request_method TEXT,
    request_path TEXT,
    response_status INTEGER,
    response_body TEXT,
    error_message TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_ai_audit_user ON ai_audit_logs(user_id);
CREATE INDEX IF NOT EXISTS idx_ai_audit_tool ON ai_audit_logs(tool_name);
CREATE INDEX IF NOT EXISTS idx_ai_audit_created ON ai_audit_logs(created_at);
CREATE INDEX IF NOT EXISTS idx_ai_audit_conversation ON ai_audit_logs(conversation_id);
`)
	return err
}
