package mysql

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

// CredentialCipher encrypts and decrypts secrets stored in the database with
// AES-256-GCM, keyed by CONNECTION_CREDENTIALS_ENCRYPTION_KEY. It produces the
// same format as ConnectionCredentialsRepository (base64 of nonce||ciphertext),
// so both can read each other's values. Used by StorageDiskCredentialsRepository;
// the connection credential / setting repositories still carry their own copy.
type CredentialCipher struct {
	key []byte
	err error
}

// NewCredentialCipher loads the key from the environment. A missing or invalid
// key is not an error here: it surfaces from Encrypt/Decrypt, so the service can
// start without it and fail only when a secret is actually needed.
func NewCredentialCipher() *CredentialCipher {
	key, err := loadConnectionCredentialsEncryptionKey()
	return &CredentialCipher{key: key, err: err}
}

func (c *CredentialCipher) aead() (cipher.AEAD, error) {
	if c.err != nil {
		return nil, c.err
	}
	if len(c.key) != 32 {
		return nil, ErrInvalidCredentialsEncryptionKey
	}
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func (c *CredentialCipher) Encrypt(value string) (string, error) {
	aead, err := c.aead()
	if err != nil {
		return "", err
	}

	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(aead.Seal(nonce, nonce, []byte(value), nil)), nil
}

func (c *CredentialCipher) Decrypt(value string) (string, error) {
	aead, err := c.aead()
	if err != nil {
		return "", err
	}

	payload, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return "", fmt.Errorf("failed to decode encrypted credential value: %w", err)
	}
	if len(payload) < aead.NonceSize() {
		return "", errors.New("invalid encrypted credential payload")
	}

	plaintext, err := aead.Open(nil, payload[:aead.NonceSize()], payload[aead.NonceSize():], nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt credential value: %w", err)
	}
	return string(plaintext), nil
}
