package config

import (
	"strings"
	"testing"
)

func TestEncryptDecryptNotifySecret(t *testing.T) {
	t.Run("roundtrip", func(t *testing.T) {
		plaintext := "my-secret-key-12345"
		enc, err := encryptNotifySecret(plaintext)
		if err != nil {
			t.Fatalf("encrypt failed: %v", err)
		}
		if !strings.HasPrefix(enc, notifySecretPrefix) {
			t.Fatalf("encrypted value should have $ENC$ prefix: %s", enc)
		}
		if enc == plaintext {
			t.Fatal("encrypted value should differ from plaintext")
		}

		dec, err := decryptNotifySecret(enc)
		if err != nil {
			t.Fatalf("decrypt failed: %v", err)
		}
		if dec != plaintext {
			t.Fatalf("roundtrip failed: got %q, want %q", dec, plaintext)
		}
	})

	t.Run("empty", func(t *testing.T) {
		enc, err := encryptNotifySecret("")
		if err != nil {
			t.Fatalf("encrypt empty failed: %v", err)
		}
		if enc != "" {
			t.Fatalf("encrypt empty should return empty: got %q", enc)
		}

		dec, err := decryptNotifySecret("")
		if err != nil {
			t.Fatalf("decrypt empty failed: %v", err)
		}
		if dec != "" {
			t.Fatalf("decrypt empty should return empty: got %q", dec)
		}
	})

	t.Run("already_encrypted", func(t *testing.T) {
		enc, err := encryptNotifySecret("$ENC$already-encrypted")
		if err != nil {
			t.Fatalf("encrypt failed: %v", err)
		}
		if enc != "$ENC$already-encrypted" {
			t.Fatal("already encrypted should pass through unchanged")
		}
	})

	t.Run("plaintext_passthrough", func(t *testing.T) {
		dec, err := decryptNotifySecret("plaintext-secret")
		if err != nil {
			t.Fatalf("decrypt plaintext failed: %v", err)
		}
		if dec != "plaintext-secret" {
			t.Fatal("plaintext should pass through unchanged")
		}
	})

	t.Run("different_ciphertexts", func(t *testing.T) {
		e1, _ := encryptNotifySecret("secret")
		e2, _ := encryptNotifySecret("secret")
		if e1 == e2 {
			t.Fatal("same plaintext should produce different ciphertexts (random nonce)")
		}
	})
}

func TestEncryptDecryptNotifySecrets(t *testing.T) {
	cfg := &Config{}
	cfg.Notifications.Channels = []NotificationChannelCfg{
		{ID: "ch1", Type: "dingtalk", Secret: "secret1"},
		{ID: "ch2", Type: "feishu", Secret: ""},
		{ID: "ch3", Type: "wecom", Secret: "secret3"},
	}

	EncryptNotifySecrets(cfg)

	if cfg.Notifications.Channels[0].Secret == "secret1" {
		t.Fatal("ch1 secret should be encrypted")
	}
	if !strings.HasPrefix(cfg.Notifications.Channels[0].Secret, "$ENC$") {
		t.Fatal("ch1 secret should have $ENC$ prefix")
	}
	if cfg.Notifications.Channels[1].Secret != "" {
		t.Fatal("ch2 empty secret should remain empty")
	}
	if !strings.HasPrefix(cfg.Notifications.Channels[2].Secret, "$ENC$") {
		t.Fatal("ch3 secret should have $ENC$ prefix")
	}

	DecryptNotifySecrets(cfg)

	if cfg.Notifications.Channels[0].Secret != "secret1" {
		t.Fatalf("ch1 secret roundtrip failed: %q", cfg.Notifications.Channels[0].Secret)
	}
	if cfg.Notifications.Channels[1].Secret != "" {
		t.Fatal("ch2 empty should remain empty")
	}
	if cfg.Notifications.Channels[2].Secret != "secret3" {
		t.Fatalf("ch3 secret roundtrip failed: %q", cfg.Notifications.Channels[2].Secret)
	}
}
