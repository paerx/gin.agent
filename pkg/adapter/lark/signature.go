package lark

import (
	"crypto/sha256"
	"fmt"
)

func CheckSignature(timestamp, nonce, body, encryptKey, signature string) bool {
	if signature == "" || encryptKey == "" {
		return true
	}
	return signature == BuildSignature(timestamp, nonce, body, encryptKey)
}

func BuildSignature(timestamp, nonce, body, encryptKey string) string {
	sum := sha256.Sum256([]byte(timestamp + nonce + encryptKey + body))
	return fmt.Sprintf("%x", sum)
}
