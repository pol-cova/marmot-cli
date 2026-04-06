package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"os"
)

// AESEncryptor implements AES-256-GCM encryption
type AESEncryptor struct {
	key []byte
}

// NewAESEncryptor creates a new AES encryptor
func NewAESEncryptor() *AESEncryptor {
	return &AESEncryptor{}
}

// GenerateKey generates a new 256-bit (32-byte) encryption key
func (e *AESEncryptor) GenerateKey() ([]byte, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}
	return key, nil
}

// LoadKey loads an encryption key
func (e *AESEncryptor) LoadKey(key []byte) error {
	if len(key) != 32 {
		return fmt.Errorf("invalid key length: expected 32 bytes, got %d", len(key))
	}
	e.key = key
	return nil
}

// LoadKeyFromFile loads an encryption key from a file
func (e *AESEncryptor) LoadKeyFromFile(path string) error {
	key, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read key file: %w", err)
	}
	
	if err := e.LoadKey(key); err != nil {
		return err
	}
	
	return nil
}

// SaveKeyToFile saves the encryption key to a file with secure permissions
func (e *AESEncryptor) SaveKeyToFile(path string) error {
	if len(e.key) == 0 {
		return fmt.Errorf("no key loaded")
	}
	
	if err := os.WriteFile(path, e.key, 0600); err != nil {
		return fmt.Errorf("failed to write key file: %w", err)
	}
	
	return nil
}

// ExportKeyAsBase64 returns the loaded key as a base64-encoded string
func (e *AESEncryptor) ExportKeyAsBase64() (string, error) {
	if len(e.key) == 0 {
		return "", fmt.Errorf("no key loaded")
	}
	return base64.StdEncoding.EncodeToString(e.key), nil
}

// ImportKeyFromBase64 loads an encryption key from a base64-encoded string
func (e *AESEncryptor) ImportKeyFromBase64(b64 string) error {
	key, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return fmt.Errorf("failed to decode base64 key: %w", err)
	}
	return e.LoadKey(key)
}

// Encrypt encrypts data from reader and writes to writer
func (e *AESEncryptor) Encrypt(plaintext io.Reader, ciphertext io.Writer) error {
	if len(e.key) == 0 {
		return fmt.Errorf("no key loaded")
	}
	
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return fmt.Errorf("failed to create cipher: %w", err)
	}
	
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("failed to create GCM: %w", err)
	}
	
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return fmt.Errorf("failed to generate nonce: %w", err)
	}
	
	// Write nonce first
	if _, err := ciphertext.Write(nonce); err != nil {
		return fmt.Errorf("failed to write nonce: %w", err)
	}
	
	// Read plaintext into buffer
	plaintextBytes, err := io.ReadAll(plaintext)
	if err != nil {
		return fmt.Errorf("failed to read plaintext: %w", err)
	}
	
	// Encrypt and write
	encrypted := gcm.Seal(nil, nonce, plaintextBytes, nil)
	if _, err := ciphertext.Write(encrypted); err != nil {
		return fmt.Errorf("failed to write encrypted data: %w", err)
	}
	
	return nil
}

// Decrypt decrypts data from reader and writes to writer
func (e *AESEncryptor) Decrypt(ciphertext io.Reader, plaintext io.Writer) error {
	if len(e.key) == 0 {
		return fmt.Errorf("no key loaded")
	}
	
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return fmt.Errorf("failed to create cipher: %w", err)
	}
	
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("failed to create GCM: %w", err)
	}
	
	// Read nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(ciphertext, nonce); err != nil {
		return fmt.Errorf("failed to read nonce: %w", err)
	}
	
	// Read encrypted data
	ciphertextBytes, err := io.ReadAll(ciphertext)
	if err != nil {
		return fmt.Errorf("failed to read ciphertext: %w", err)
	}
	
	// Decrypt
	decrypted, err := gcm.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return fmt.Errorf("failed to decrypt: %w", err)
	}
	
	// Write decrypted data
	if _, err := plaintext.Write(decrypted); err != nil {
		return fmt.Errorf("failed to write decrypted data: %w", err)
	}
	
	return nil
}

