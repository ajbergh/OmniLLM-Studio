package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

const (
	keySize      = 32 // AES-256
	envKeyName   = "OMNILLM_MASTER_KEY"
	seedFileName = ".omnillm_key"
)

var (
	masterKey     []byte
	masterKeyOnce sync.Once
	masterKeyErr  error
)

// getMasterKey returns the 32-byte master key, loading it lazily.
// Priority: 1) OMNILLM_MASTER_KEY env var (hex-encoded), 2) machine-scoped seed file.
func getMasterKey() ([]byte, error) {
	masterKeyOnce.Do(func() {
		// 1. Try environment variable (hex-encoded 32 bytes = 64 hex chars)
		if envKey := os.Getenv(envKeyName); envKey != "" {
			decoded, err := hex.DecodeString(envKey)
			if err != nil || len(decoded) != keySize {
				masterKeyErr = fmt.Errorf("invalid %s: must be 64 hex characters (32 bytes)", envKeyName)
				return
			}
			masterKey = decoded
			return
		}

		// 2. Machine-scoped seed file in user config directory
		configDir, err := os.UserConfigDir()
		if err != nil {
			masterKeyErr = fmt.Errorf("cannot determine config dir: %w", err)
			return
		}

		seedDir := filepath.Join(configDir, "omnillm-studio")
		seedPath := filepath.Join(seedDir, seedFileName)

		// Try reading existing seed
		if data, err := os.ReadFile(seedPath); err == nil {
			decoded, err := hex.DecodeString(string(data))
			if err == nil && len(decoded) == keySize {
				masterKey = decoded
				return
			}
		}

		// Generate new key and write seed file
		key := make([]byte, keySize)
		if _, err := io.ReadFull(rand.Reader, key); err != nil {
			masterKeyErr = fmt.Errorf("generate random key: %w", err)
			return
		}

		if err := os.MkdirAll(seedDir, 0700); err != nil {
			masterKeyErr = fmt.Errorf("create config dir: %w", err)
			return
		}
		if err := os.WriteFile(seedPath, []byte(hex.EncodeToString(key)), 0600); err != nil {
			masterKeyErr = fmt.Errorf("write seed file: %w", err)
			return
		}

		masterKey = key
	})

	return masterKey, masterKeyErr
}

// Encrypt encrypts plaintext using AES-256-GCM and returns a base64-encoded ciphertext.
// The nonce is prepended to the ciphertext.
func Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	key, err := getMasterKey()
	if err != nil {
		return "", fmt.Errorf("get master key: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	// Seal appends ciphertext to nonce, so result = nonce + ciphertext + tag
	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// Decrypt decrypts a base64-encoded AES-256-GCM ciphertext (with prepended nonce).
// Returns an explicit error if the data cannot be decrypted.
func Decrypt(encoded string) (string, error) {
	if encoded == "" {
		return "", nil
	}

	key, err := getMasterKey()
	if err != nil {
		return "", fmt.Errorf("get master key: %w", err)
	}

	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("base64 decode failed: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt failed: %w", err)
	}

	return string(plaintext), nil
}

// DecryptOrPlaintext attempts to decrypt the value. If decryption fails (e.g.
// the value is legacy plaintext stored before encryption was enabled), the
// original value is returned unchanged. Use this only during migration paths;
// prefer Decrypt for normal operation.
func DecryptOrPlaintext(encoded string) string {
	if encoded == "" {
		return ""
	}
	decrypted, err := Decrypt(encoded)
	if err != nil {
		return encoded
	}
	return decrypted
}

// IsEncrypted checks if a value looks like it was encrypted (base64 with minimum length).
func IsEncrypted(value string) bool {
	if value == "" {
		return false
	}
	data, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return false
	}
	// AES-256-GCM: nonce (12) + at least 1 byte ciphertext + tag (16) = 29 minimum
	return len(data) >= 29
}
