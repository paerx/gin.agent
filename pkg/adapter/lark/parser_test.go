package lark

import "testing"

func TestParseAgentInput(t *testing.T) {
	req := EventRequest{}
	req.Event.Message.MessageType = "text"
	req.Event.Message.ChatType = "p2p"
	req.Event.Message.ChatID = "chat1"
	req.Event.Message.MessageID = "msg1"
	req.Event.Message.Content = `{"text":"查一下 0xabc 用户信息"}`
	req.Event.Sender.SenderID.UserID = "u1"

	input, ignore, err := ParseAgentInput(req)
	if err != nil {
		t.Fatalf("ParseAgentInput() error = %v", err)
	}
	if ignore {
		t.Fatal("expected message not ignored")
	}
	if input.UserID != "u1" || input.ChatType != "p2p" {
		t.Fatalf("input = %#v", input)
	}
}

func TestParseAgentInputIgnoreBot(t *testing.T) {
	req := EventRequest{}
	req.Event.Message.MessageType = "text"
	req.Event.Sender.SenderType = "bot"
	_, ignore, err := ParseAgentInput(req)
	if err != nil {
		t.Fatalf("ParseAgentInput() error = %v", err)
	}
	if !ignore {
		t.Fatal("expected bot message ignored")
	}
}
