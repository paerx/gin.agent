package lark

import "context"

type Sender interface {
	SendText(ctx context.Context, chatID string, text string) error
}
