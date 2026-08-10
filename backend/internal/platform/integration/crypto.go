package integration

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
)

// ErrNoEncryptionKey is returned when a connector needs to store a credential
// and the deployment has not configured a key to seal it with.
//
// This fails the write rather than storing the credential in the clear. A
// refresh token for someone's Google Drive sitting in plaintext in a database
// backup is worse than a connector that could not be saved, and the second
// failure is the one an operator can see and fix.
var ErrNoEncryptionKey = errors.New(
	"INTEGRATION_ENCRYPTION_KEY is not set, so credentials cannot be stored safely")

const encryptionKeyEnv = "INTEGRATION_ENCRYPTION_KEY"

var (
	keyOnce sync.Once
	keyVal  []byte
	keyErr  error
)

// encryptionKey resolves the 32-byte AES key once per process.
//
// The variable is accepted as base64, hex, or raw text. Raw text is hashed to
// the required length rather than rejected, because an operator who sets a
// passphrase should get a working deployment, not a startup failure they have
// to decode an error message to understand — and hashing is what they meant.
func encryptionKey() ([]byte, error) {
	keyOnce.Do(func() {
		raw := strings.TrimSpace(os.Getenv(encryptionKeyEnv))
		if raw == "" {
			keyErr = ErrNoEncryptionKey
			return
		}
		if decoded, err := base64.StdEncoding.DecodeString(raw); err == nil && len(decoded) == 32 {
			keyVal = decoded
			return
		}
		if decoded, err := hex.DecodeString(raw); err == nil && len(decoded) == 32 {
			keyVal = decoded
			return
		}
		sum := sha256.Sum256([]byte(raw))
		keyVal = sum[:]
	})
	return keyVal, keyErr
}

// EncryptionConfigured reports whether credentials can be stored. The
// integrations screen asks so it can say why a provider is unavailable instead
// of failing at the moment of saving.
func EncryptionConfigured() bool {
	_, err := encryptionKey()
	return err == nil
}

// seal encrypts plaintext with AES-256-GCM. The nonce is prepended to the
// ciphertext, which is how open finds it again.
func seal(plaintext []byte) ([]byte, error) {
	if len(plaintext) == 0 {
		return nil, nil
	}
	key, err := encryptionKey()
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("integration: cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("integration: gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("integration: nonce: %w", err)
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// open reverses seal. A ciphertext that does not authenticate is an error, not
// an empty credential: silently returning "" would turn a rotated or corrupted
// key into unsigned webhooks rather than a visible failure.
func open(ciphertext []byte) ([]byte, error) {
	if len(ciphertext) == 0 {
		return nil, nil
	}
	key, err := encryptionKey()
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("integration: cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("integration: gcm: %w", err)
	}
	if len(ciphertext) < gcm.NonceSize() {
		return nil, errors.New("integration: stored credential is truncated")
	}
	nonce, body := ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, body, nil)
	if err != nil {
		return nil, fmt.Errorf("integration: stored credential could not be decrypted "+
			"(has %s changed?): %w", encryptionKeyEnv, err)
	}
	return plaintext, nil
}
