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
	"strings"
	"sync"
)

const (
	keySize           = 32 // AES-256
	envKeyName        = "OMNILLM_MASTER_KEY"
	envRequireKeyName = "OMNILLM_REQUIRE_MASTER_KEY"
	seedFileName      = ".omnillm_key"
)

var (
	masterKey     []byte
	masterKeyOnce sync.Once
	masterKeyErr  error
)

// getMasterKey returns the 32-byte master key, loading it lazily.
// Priority: 1) OMNILLM_MASTER_KEY, 2) a machine-scoped seed file for local
// desktop/server use. Container and other persistent deployments should set
// OMNILLM_REQUIRE_MASTER_KEY=true so a replacement process can never silently
// generate a different key beside an existing database.
func getMasterKey() ([]byte, error) {
	masterKeyOnce.Do(func() {
		if envKey := strings.TrimSpace(os.Getenv(envKeyName)); envKey != "" {
			decoded, err := hex.DecodeString(envKey)
			if err != nil || len(decoded) != keySize {
				masterKeyErr = fmt.Errorf("invalid %s: must be 64 hex characters (32 bytes)", envKeyName)
				return
			}
			masterKey = decoded
			return
		}

		if strings.EqualFold(strings.TrimSpace(os.Getenv(envRequireKeyName)), "true") {
			masterKeyErr = fmt.Errorf("%s is required when %s=true", envKeyName, envRequireKeyName)
			return
		}

		configDir, err := os.UserConfigDir()
		if err != nil {
			masterKeyErr = fmt.Errorf("cannot determine config dir: %w", err)
			return
		}
		seedDir := filepath.Join(configDir, "omnillm-studio")
		seedPath := filepath.Join(seedDir, seedFileName)

		if data, err := os.ReadFile(seedPath); err == nil {
			decoded, decodeErr := hex.DecodeString(strings.TrimSpace(string(data)))
			if decodeErr == nil && len(decoded) == keySize {
				masterKey = decoded
				return
			}
			masterKeyErr = fmt.Errorf("existing master-key seed at %s is invalid", seedPath)
			return
		} else if !os.IsNotExist(err) {
			masterKeyErr = fmt.Errorf("read master-key seed: %w", err)
			return
		}

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
	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// Decrypt decrypts a base64-encoded AES-256-GCM ciphertext (with prepended nonce).
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
	if len(data) < gcm.NonceSize() {
		return "", fmt.Errorf("ciphertext too short")
	}
	nonce, ciphertext := data[:gcm.NonceSize()], data[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt failed: %w", err)
	}
	return string(plaintext), nil
}

// DecryptOrPlaintext is restricted to legacy migration paths.
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

// IsEncrypted checks if a value looks like AES-GCM data encoded as base64.
func IsEncrypted(value string) bool {
	if value == "" {
		return false
	}
	data, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return false
	}
	return len(data) >= 29
}
