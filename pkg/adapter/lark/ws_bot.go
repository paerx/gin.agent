package lark

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"gin.agent/pkg/agent"

	larkevent "github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"
)

type LongConnectionBot struct {
	agent             agent.Agent
	sender            Sender
	appID             string
	appSecret         string
	verificationToken string
	encryptKey        string
	domain            string
}

func NewLongConnectionBot(agent agent.Agent, sender Sender, appID, appSecret, verificationToken, encryptKey string) *LongConnectionBot {
	return NewLongConnectionBotWithDomain(agent, sender, appID, appSecret, verificationToken, encryptKey, "")
}

func NewLongConnectionBotWithDomain(agent agent.Agent, sender Sender, appID, appSecret, verificationToken, encryptKey, domain string) *LongConnectionBot {
	return &LongConnectionBot{
		agent:             agent,
		sender:            sender,
		appID:             appID,
		appSecret:         appSecret,
		verificationToken: verificationToken,
		encryptKey:        encryptKey,
		domain:            strings.TrimRight(domain, "/"),
	}
}

func (b *LongConnectionBot) Start(ctx context.Context) error {
	if strings.TrimSpace(b.appID) == "" || strings.TrimSpace(b.appSecret) == "" {
		return fmt.Errorf("lark long connection requires LARK_APP_ID and LARK_APP_SECRET")
	}

	dispatcher := larkevent.NewEventDispatcher(b.verificationToken, b.encryptKey).
		OnP2MessageReceiveV1(func(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
			input, ignore, err := AgentInputFromP2MessageReceiveV1(event)
			if err != nil {
				return err
			}
			if ignore || input == nil {
				return nil
			}
			log.Printf(
				"[lark event] platform=%s chat_id=%s chat_type=%s user_id=%s message_id=%s text=%q",
				input.Platform,
				input.ChatID,
				input.ChatType,
				input.UserID,
				input.MessageID,
				input.Text,
			)
			output, err := b.agent.HandleMessage(ctx, *input)
			if err != nil {
				return err
			}
			if output != nil && output.Text != "" && b.sender != nil {
				return b.sender.SendText(ctx, input.ChatID, output.Text)
			}
			return nil
		})

	options := []larkws.ClientOption{larkws.WithEventHandler(dispatcher)}
	if b.domain != "" {
		options = append(options, larkws.WithDomain(b.domain))
	}
	client := larkws.NewClient(b.appID, b.appSecret, options...)
	return client.Start(ctx)
}

func AgentInputFromP2MessageReceiveV1(event *larkim.P2MessageReceiveV1) (*agent.AgentInput, bool, error) {
	if event == nil || event.Event == nil || event.Event.Message == nil {
		return nil, false, fmt.Errorf("empty lark message event")
	}
	message := event.Event.Message
	if ptrValue(message.MessageType) != "text" {
		return nil, false, nil
	}
	if event.Event.Sender != nil && ptrValue(event.Event.Sender.SenderType) == "bot" {
		return nil, true, nil
	}

	var content textContent
	if err := json.Unmarshal([]byte(ptrValue(message.Content)), &content); err != nil {
		return nil, false, fmt.Errorf("parse lark text content: %w", err)
	}

	userID := ""
	if event.Event.Sender != nil && event.Event.Sender.SenderId != nil {
		userID = ptrValue(event.Event.Sender.SenderId.UserId)
		if userID == "" {
			userID = ptrValue(event.Event.Sender.SenderId.OpenId)
		}
	}
	if userID == "" {
		return nil, false, fmt.Errorf("lark sender user_id is empty")
	}

	return &agent.AgentInput{
		Platform:  "lark",
		ChatID:    ptrValue(message.ChatId),
		ChatType:  normalizeChatType(ptrValue(message.ChatType)),
		UserID:    userID,
		MessageID: ptrValue(message.MessageId),
		Text:      strings.TrimSpace(content.Text),
	}, false, nil
}

func ptrValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
