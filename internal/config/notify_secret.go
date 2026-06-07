package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/unimap/project/internal/logger"
)

const (
	legacyNotifyPepper = "unimap-notify-channel-secret-v1"
	notifySecretPrefix = "$ENC$"
	notifySecretV2Sep  = "$ENC$v2:"
	pepperEnvVar       = "UNIMAP_NOTIFY_PEPPER"
)

var (
	notifyPepper     string
	notifyPepperOnce sync.Once
)

func initNotifyPepper() {
	if env := os.Getenv(pepperEnvVar); env != "" {
		notifyPepper = env
		return
	}
	notifyPepper = legacyNotifyPepper
	logger.Warnf("UNIMAP_NOTIFY_PEPPER not set, using legacy pepper — set the env var for production deployments")
}

func getNotifyPepper() string {
	notifyPepperOnce.Do(initNotifyPepper)
	return notifyPepper
}

// ResetNotifyPepperForTest resets the pepper for testing.
func ResetNotifyPepperForTest() {
	notifyPepper = ""
	notifyPepperOnce = sync.Once{}
}

func deriveNotifyKey() []byte {
	h := sha256.Sum256([]byte(getNotifyPepper()))
	return h[:]
}

func pepperID() string {
	h := sha256.Sum256([]byte(getNotifyPepper()))
	return fmt.Sprintf("%x", h[:4])
}

func encryptNotifySecret(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	if strings.HasPrefix(plaintext, notifySecretPrefix) {
		return plaintext, nil
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
	encoded := base64.URLEncoding.EncodeToString(ciphertext)
	return notifySecretV2Sep + pepperID() + ":" + encoded, nil
}

func decryptNotifySecret(encoded string) (string, error) {
	if encoded == "" || !strings.HasPrefix(encoded, notifySecretPrefix) {
		return encoded, nil
	}

	// v2 format: $ENC$v2:<pepper_id>:<base64>
	if strings.HasPrefix(encoded, notifySecretV2Sep) {
		return decryptV2Secret(encoded)
	}

	// v1 format: $ENC$<base64> — try current pepper, then legacy
	data := strings.TrimPrefix(encoded, notifySecretPrefix)
	ciphertext, err := base64.URLEncoding.DecodeString(data)
	if err != nil {
		return "", fmt.Errorf("base64 decode: %w", err)
	}

	// Try current pepper first
	plain, err := decryptWithKey(ciphertext, deriveNotifyKey())
	if err == nil {
		return plain, nil
	}

	// If current pepper != legacy, try legacy
	if getNotifyPepper() != legacyNotifyPepper {
		h := sha256.Sum256([]byte(legacyNotifyPepper))
		plain, err = decryptWithKey(ciphertext, h[:])
		if err == nil {
			return plain, nil
		}
	}

	return "", fmt.Errorf("decrypt v1 secret: %w", err)
}

func decryptV2Secret(encoded string) (string, error) {
	rest := strings.TrimPrefix(encoded, notifySecretV2Sep)
	parts := strings.SplitN(rest, ":", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid v2 format")
	}
	storedID := parts[0]
	data := parts[1]

	if storedID != pepperID() {
		return "", fmt.Errorf("pepper ID mismatch (stored=%s current=%s), secret needs migration", storedID, pepperID())
	}

	ciphertext, err := base64.URLEncoding.DecodeString(data)
	if err != nil {
		return "", fmt.Errorf("base64 decode: %w", err)
	}
	return decryptWithKey(ciphertext, deriveNotifyKey())
}

func decryptWithKey(ciphertext, key []byte) (string, error) {
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

// EncryptNotifySecrets encrypts the Secret field for every notification channel.
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
			logger.Errorf("failed to encrypt secret for channel %s: %v", ch.ID, err)
			continue
		}
		ch.Secret = enc
	}
}

// DecryptNotifySecrets decrypts the Secret field for every notification channel.
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
			continue
		}
		ch.Secret = dec
	}
}

// NeedsPepperMigration checks if any encrypted secrets use a different pepper.
func NeedsPepperMigration(cfg *Config) bool {
	if cfg == nil {
		return false
	}
	for _, ch := range cfg.Notifications.Channels {
		if ch.Secret == "" || !strings.HasPrefix(ch.Secret, notifySecretPrefix) {
			continue
		}
		if strings.HasPrefix(ch.Secret, notifySecretV2Sep) {
			rest := strings.TrimPrefix(ch.Secret, notifySecretV2Sep)
			parts := strings.SplitN(rest, ":", 2)
			if len(parts) == 2 && parts[0] != pepperID() {
				return true
			}
		} else {
			// v1 format — needs migration if pepper changed
			if getNotifyPepper() != legacyNotifyPepper {
				return true
			}
		}
	}
	return false
}

// MigrateNotifySecrets re-encrypts all secrets with the current pepper.
func MigrateNotifySecrets(cfg *Config) error {
	if cfg == nil {
		return nil
	}
	for i := range cfg.Notifications.Channels {
		ch := &cfg.Notifications.Channels[i]
		if ch.Secret == "" || !strings.HasPrefix(ch.Secret, notifySecretPrefix) {
			continue
		}
		dec, err := decryptNotifySecret(ch.Secret)
		if err != nil {
			return fmt.Errorf("migrate channel %s: %w", ch.ID, err)
		}
		ch.Secret = dec // set to plaintext temporarily
		enc, err := encryptNotifySecret(ch.Secret)
		if err != nil {
			return fmt.Errorf("re-encrypt channel %s: %w", ch.ID, err)
		}
		ch.Secret = enc
	}
	return nil
}

// GenerateRandomPepper generates a cryptographically random pepper string.
func GenerateRandomPepper() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate random pepper: %w", err)
	}
	return base64.URLEncoding.EncodeToString(b), nil
}
