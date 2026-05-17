package lark

import (
	"testing"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

func TestAgentInputFromP2MessageReceiveV1(t *testing.T) {
	messageID := "msg-1"
	chatID := "chat-1"
	chatType := "p2p"
	messageType := "text"
	content := `{"text":"查一下 0xabc 用户信息"}`
	senderType := "user"
	userID := "user-1"

	input, ignore, err := AgentInputFromP2MessageReceiveV1(&larkim.P2MessageReceiveV1{
		Event: &larkim.P2MessageReceiveV1Data{
			Sender: &larkim.EventSender{
				SenderType: &senderType,
				SenderId: &larkim.UserId{
					UserId: &userID,
				},
			},
			Message: &larkim.EventMessage{
				MessageId:   &messageID,
				ChatId:      &chatID,
				ChatType:    &chatType,
				MessageType: &messageType,
				Content:     &content,
			},
		},
	})
	if err != nil {
		t.Fatalf("AgentInputFromP2MessageReceiveV1() error = %v", err)
	}
	if ignore {
		t.Fatal("expected event not ignored")
	}
	if input.UserID != userID || input.MessageID != messageID || input.Text != "查一下 0xabc 用户信息" {
		t.Fatalf("input = %#v", input)
	}
}
