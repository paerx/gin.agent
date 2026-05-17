package lark

import (
	"encoding/json"
	"io"
	"log"
	"net/http"

	"gin.agent/pkg/agent"

	"github.com/gin-gonic/gin"
)

type Bot struct {
	agent             agent.Agent
	sender            Sender
	verificationToken string
	encryptKey        string
}

func NewBot(agent agent.Agent, sender Sender, verificationToken, encryptKey string) *Bot {
	return &Bot{
		agent:             agent,
		sender:            sender,
		verificationToken: verificationToken,
		encryptKey:        encryptKey,
	}
}

func (b *Bot) HandleEvent(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var event EventRequest
	if err := json.Unmarshal(body, &event); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if event.Type == "url_verification" {
		c.JSON(http.StatusOK, gin.H{"challenge": event.Challenge})
		return
	}
	if token := event.Header.Token; b.verificationToken != "" && token != "" && token != b.verificationToken {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid verification token"})
		return
	}
	if !CheckSignature(
		c.GetHeader("X-Lark-Request-Timestamp"),
		c.GetHeader("X-Lark-Request-Nonce"),
		string(body),
		b.encryptKey,
		c.GetHeader("X-Lark-Signature"),
	) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid signature"})
		return
	}

	input, ignore, err := ParseAgentInput(event)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if ignore || input == nil {
		c.JSON(http.StatusOK, gin.H{"ok": true})
		return
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

	output, err := b.agent.HandleMessage(c.Request.Context(), *input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if output != nil && output.Text != "" && b.sender != nil {
		if err := b.sender.SendText(c.Request.Context(), input.ChatID, output.Text); err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
