package encryption

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"vpn-backend/internal/domain"
)

const nonceSize = 12

type PrivateKeyCipher struct {
	aead cipher.AEAD
}

var _ domain.PrivateKeyCipher = (*PrivateKeyCipher)(nil)

func NewPrivateKeyCipher(key []byte) (*PrivateKeyCipher, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create aes cipher: %w", err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create aes-gcm: %w", err)
	}

	if aead.NonceSize() != nonceSize {
		return nil, fmt.Errorf("unexpected nonce size: %d", aead.NonceSize())
	}

	return &PrivateKeyCipher{aead: aead}, nil
}

func (c *PrivateKeyCipher) Encrypt(_ context.Context, plaintext string) (string, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext := c.aead.Seal(nil, nonce, []byte(plaintext), nil)
	payload := append(nonce, ciphertext...)

	return base64.StdEncoding.EncodeToString(payload), nil
}

func (c *PrivateKeyCipher) Decrypt(_ context.Context, ciphertext string) (string, error) {
	payload, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("decode ciphertext: %w", err)
	}

	if len(payload) < c.aead.NonceSize() {
		return "", fmt.Errorf("ciphertext is too short")
	}

	nonce := payload[:c.aead.NonceSize()]
	encrypted := payload[c.aead.NonceSize():]

	plaintext, err := c.aead.Open(nil, nonce, encrypted, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt ciphertext: %w", err)
	}

	return string(plaintext), nil
}
