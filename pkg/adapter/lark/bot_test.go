package lark

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestHandleEventEncryptedChallenge(t *testing.T) {
	gin.SetMode(gin.TestMode)

	encryptKey := "test-encrypt-key"
	plain := `{"type":"url_verification","token":"verify-token","challenge":"challenge-code"}`
	encrypted, err := encryptLarkEventForTest(plain, encryptKey)
	if err != nil {
		t.Fatalf("encryptLarkEventForTest() error = %v", err)
	}

	router := gin.New()
	router.POST("/lark/events", NewBot(nil, nil, "verify-token", encryptKey).HandleEvent)

	req := httptest.NewRequest(http.MethodPost, "/lark/events", strings.NewReader(`{"encrypt":"`+encrypted+`"}`))
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.Code, resp.Body.String())
	}
	if got := resp.Body.String(); got != `{"challenge":"challenge-code"}` {
		t.Fatalf("body = %s", got)
	}
}

func TestCheckSignature(t *testing.T) {
	body := `{"hello":"world"}`
	signature := BuildSignature("1", "nonce", body, "encrypt-key")
	if !CheckSignature("1", "nonce", body, "encrypt-key", signature) {
		t.Fatal("expected signature to pass")
	}
	if CheckSignature("2", "nonce", body, "encrypt-key", signature) {
		t.Fatal("expected signature to fail")
	}
}

func encryptLarkEventForTest(plain, encryptKey string) (string, error) {
	key := sha256.Sum256([]byte(encryptKey))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", err
	}
	iv := bytes.Repeat([]byte{1}, aes.BlockSize)
	payload := pkcs7PadForTest([]byte(plain), aes.BlockSize)
	cipherText := make([]byte, aes.BlockSize+len(payload))
	copy(cipherText, iv)
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(cipherText[aes.BlockSize:], payload)
	return base64.StdEncoding.EncodeToString(cipherText), nil
}

func pkcs7PadForTest(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	out := make([]byte, len(data)+padding)
	copy(out, data)
	copy(out[len(data):], bytes.Repeat([]byte{byte(padding)}, padding))
	return out
}
