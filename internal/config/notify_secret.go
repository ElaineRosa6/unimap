package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"strings"
)

const (
	notifySecretPepper = "unimap-notify-channel-secret-v1"
	notifySecretPrefix = "$ENC$"
)

// deriveNotifyKey returns a 32-byte AES key from the project pepper.
func deriveNotifyKey() []byte {
	h := sha256.Sum256([]byte(notifySecretPepper))
	return h[:]
}

// encryptNotifySecret encrypts a plaintext secret with AES-GCM and returns a
// base64-encoded string prefixed with "$ENC$". Empty strings are returned as-is.
func encryptNotifySecret(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	if strings.HasPrefix(plaintext, notifySecretPrefix) {
		return plaintext, nil // already encrypted
	}

	key := deriveNotifyKey()
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("aes new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("new gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return notifySecretPrefix + base64.URLEncoding.EncodeToString(ciphertext), nil
}

// decryptNotifySecret decrypts a string produced by encryptNotifySecret.
// Plaintext strings (no $ENC$ prefix) and empty strings are returned as-is.
func decryptNotifySecret(encoded string) (string, error) {
	if encoded == "" || !strings.HasPrefix(encoded, notifySecretPrefix) {
		return encoded, nil
	}
	data := strings.TrimPrefix(encoded, notifySecretPrefix)

	key := deriveNotifyKey()
	ciphertext, err := base64.URLEncoding.DecodeString(data)
	if err != nil {
		return "", fmt.Errorf("base64 decode: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("aes new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("new gcm: %w", err)
	}
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("gcm open: %w", err)
	}
	return string(plaintext), nil
}

// EncryptNotifySecrets encrypts the Secret field for every notification channel
// in the config that has a non-empty secret. Call before persisting config to disk.
func EncryptNotifySecrets(cfg *Config) {
	if cfg == nil {
		return
	}
	for i := range cfg.Notifications.Channels {
		ch := &cfg.Notifications.Channels[i]
		if ch.Secret == "" {
			continue
		}
		enc, err := encryptNotifySecret(ch.Secret)
		if err != nil {
			log.Printf("[notify] failed to encrypt secret for channel %s: %v", ch.ID, err)
			continue
		}
		ch.Secret = enc
	}
}

// DecryptNotifySecrets decrypts the Secret field for every notification channel
// in the config. Call after loading config from disk.
func DecryptNotifySecrets(cfg *Config) {
	if cfg == nil {
		return
	}
	for i := range cfg.Notifications.Channels {
		ch := &cfg.Notifications.Channels[i]
		if ch.Secret == "" || !strings.HasPrefix(ch.Secret, notifySecretPrefix) {
			continue
		}
		dec, err := decryptNotifySecret(ch.Secret)
		if err != nil {
			continue // keep original on error
		}
		ch.Secret = dec
	}
}
