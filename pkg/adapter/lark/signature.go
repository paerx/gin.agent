package lark

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
)

func CheckSignature(timestamp, nonce, body, encryptKey, signature string) bool {
	if signature == "" || encryptKey == "" {
		return true
	}
	mac := hmac.New(sha256.New, []byte(encryptKey))
	mac.Write([]byte(timestamp))
	mac.Write([]byte(nonce))
	mac.Write([]byte(body))
	expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(signature), []byte(expected))
}
