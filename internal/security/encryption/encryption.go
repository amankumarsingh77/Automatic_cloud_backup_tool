package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"os"
)

type EncryptionManager struct {
	key []byte
}

func NewEncryptionManager(masterPassword string) (*EncryptionManager, error) {
	
	hash := sha256.Sum256([]byte(masterPassword))
	return &EncryptionManager{key: hash[:]}, nil
}

func (em *EncryptionManager) Encrypt(data []byte) ([]byte, error) {
	block, err := aes.NewCipher(em.key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	
	ciphertext := gcm.Seal(nonce, nonce, data, nil)
	
	
	encoded := make([]byte, base64.StdEncoding.EncodedLen(len(ciphertext)))
	base64.StdEncoding.Encode(encoded, ciphertext)
	return encoded, nil
}

func (em *EncryptionManager) Decrypt(encodedData []byte) ([]byte, error) {
	
	decoded := make([]byte, base64.StdEncoding.DecodedLen(len(encodedData)))
	n, err := base64.StdEncoding.Decode(decoded, encodedData)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64: %w", err)
	}
	decoded = decoded[:n]

	block, err := aes.NewCipher(em.key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	if len(decoded) < gcm.NonceSize() {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := decoded[:gcm.NonceSize()], decoded[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	return plaintext, nil
}

func (em *EncryptionManager) EncryptFile(sourcePath string) (string, error) {
	
	plaintext, err := os.ReadFile(sourcePath)
	if err != nil {
		return "", fmt.Errorf("failed to read source file: %w", err)
	}

	
	encrypted, err := em.Encrypt(plaintext)
	if err != nil {
		return "", fmt.Errorf("failed to encrypt data: %w", err)
	}

	
	encryptedPath := sourcePath + ".encrypted"

	
	if err := os.WriteFile(encryptedPath, encrypted, 0600); err != nil {
		return "", fmt.Errorf("failed to write encrypted file: %w", err)
	}

	return encryptedPath, nil
}

func (em *EncryptionManager) DecryptFile(encryptedPath string) (string, error) {
	
	ciphertext, err := os.ReadFile(encryptedPath)
	if err != nil {
		return "", fmt.Errorf("failed to read encrypted file: %w", err)
	}

	
	decrypted, err := em.Decrypt(ciphertext)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt file: %w", err)
	}

	
	decryptedPath := encryptedPath + ".decrypted"

	
	if err := os.WriteFile(decryptedPath, decrypted, 0600); err != nil {
		return "", fmt.Errorf("failed to write decrypted file: %w", err)
	}

	return decryptedPath, nil
}


func GenerateRandomKey() (string, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return "", fmt.Errorf("failed to generate random key: %w", err)
	}
	return base64.StdEncoding.EncodeToString(key), nil
}
