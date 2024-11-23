package encryption

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEncryptionManager(t *testing.T) {
	
	content := []byte("test data for encryption")
	tmpDir := os.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer os.Remove(testFile)

	
	em, err := NewEncryptionManager("test-password")
	if err != nil {
		t.Fatalf("Failed to create encryption manager: %v", err)
	}

	
	encryptedPath, err := em.EncryptFile(testFile)
	if err != nil {
		t.Fatalf("Failed to encrypt file: %v", err)
	}
	defer os.Remove(encryptedPath)

	
	encryptedContent, err := os.ReadFile(encryptedPath)
	if err != nil {
		t.Fatalf("Failed to read encrypted file: %v", err)
	}
	if string(encryptedContent) == string(content) {
		t.Error("Encrypted content should be different from original content")
	}

	
	decryptedPath, err := em.DecryptFile(encryptedPath)
	if err != nil {
		t.Fatalf("Failed to decrypt file: %v", err)
	}
	defer os.Remove(decryptedPath)

	
	decryptedContent, err := os.ReadFile(decryptedPath)
	if err != nil {
		t.Fatalf("Failed to read decrypted file: %v", err)
	}
	if string(decryptedContent) != string(content) {
		t.Error("Decrypted content does not match original content")
	}
}

func TestGenerateRandomKey(t *testing.T) {
	
	key1, err := GenerateRandomKey()
	if err != nil {
		t.Fatalf("Failed to generate first key: %v", err)
	}

	key2, err := GenerateRandomKey()
	if err != nil {
		t.Fatalf("Failed to generate second key: %v", err)
	}

	if key1 == key2 {
		t.Error("Generated keys should be different")
	}

	
	if len(key1) < 32 {
		t.Error("Generated key is too short")
	}
}
