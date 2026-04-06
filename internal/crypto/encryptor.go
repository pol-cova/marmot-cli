package crypto

import (
	"io"
)

// Encryptor defines the interface for encryption operations
type Encryptor interface {
	// Encrypt encrypts data from reader and writes to writer
	Encrypt(plaintext io.Reader, ciphertext io.Writer) error

	// Decrypt decrypts data from reader and writes to writer
	Decrypt(ciphertext io.Reader, plaintext io.Writer) error

	// GenerateKey generates a new encryption key
	GenerateKey() ([]byte, error)

	// LoadKey loads an encryption key from bytes
	LoadKey(key []byte) error

	// ExportKeyAsBase64 returns the loaded key as a base64 string for safe storage
	ExportKeyAsBase64() (string, error)

	// ImportKeyFromBase64 loads a key from a base64 string
	ImportKeyFromBase64(b64 string) error
}

