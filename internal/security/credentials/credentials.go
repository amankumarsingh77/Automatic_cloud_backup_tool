package credentials

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/amankumarsingh77/automated_backup_tool/internal/security/encryption"
)

type Credential struct {
	Provider    string `json:"provider"`
	Key         string `json:"key"`
	Secret      string `json:"secret"`
	RedirectURL string `json:"redirect_url"`
}

type CredentialManager struct {
	encryptionManager *encryption.EncryptionManager
	credentialsFile   string
	mutex            sync.RWMutex
}

func NewCredentialManager(masterPassword string) (*CredentialManager, error) {
	encManager, err := encryption.NewEncryptionManager(masterPassword)
	if err != nil {
		return nil, fmt.Errorf("failed to create encryption manager: %w", err)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	credentialsDir := filepath.Join(homeDir, ".backup")
	if err := os.MkdirAll(credentialsDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create credentials directory: %w", err)
	}

	return &CredentialManager{
		encryptionManager: encManager,
		credentialsFile:   filepath.Join(credentialsDir, "credentials.enc"),
	}, nil
}

func (cm *CredentialManager) StoreCredential(cred Credential) error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	
	creds := make(map[string]Credential)
	if _, err := os.Stat(cm.credentialsFile); err == nil {
		data, err := cm.encryptionManager.Decrypt(readFileBytes(cm.credentialsFile))
		if err == nil {
			if err := json.Unmarshal(data, &creds); err != nil {
				return fmt.Errorf("failed to parse existing credentials: %w", err)
			}
		}
	}

	
	creds[cred.Provider] = cred

	
	data, err := json.Marshal(creds)
	if err != nil {
		return fmt.Errorf("failed to marshal credentials: %w", err)
	}

	
	encrypted, err := cm.encryptionManager.Encrypt(data)
	if err != nil {
		return fmt.Errorf("failed to encrypt credentials: %w", err)
	}

	if err := os.WriteFile(cm.credentialsFile, encrypted, 0600); err != nil {
		return fmt.Errorf("failed to write credentials file: %w", err)
	}

	return nil
}

func (cm *CredentialManager) GetCredential(provider string) (*Credential, error) {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	if _, err := os.Stat(cm.credentialsFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("no credentials found")
	}

	data, err := cm.encryptionManager.Decrypt(readFileBytes(cm.credentialsFile))
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt credentials: %w", err)
	}

	var creds map[string]Credential
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("failed to parse credentials: %w", err)
	}

	cred, exists := creds[provider]
	if !exists {
		return nil, fmt.Errorf("no credentials found for provider: %s", provider)
	}

	return &cred, nil
}

func readFileBytes(path string) []byte {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return data
}


func (cm *CredentialManager) DeleteCredential(provider string) error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	if _, err := os.Stat(cm.credentialsFile); os.IsNotExist(err) {
		return nil 
	}

	data, err := cm.encryptionManager.Decrypt(readFileBytes(cm.credentialsFile))
	if err != nil {
		return fmt.Errorf("failed to decrypt credentials: %w", err)
	}

	var creds map[string]Credential
	if err := json.Unmarshal(data, &creds); err != nil {
		return fmt.Errorf("failed to parse credentials: %w", err)
	}

	delete(creds, provider)

	
	data, err = json.Marshal(creds)
	if err != nil {
		return fmt.Errorf("failed to marshal credentials: %w", err)
	}

	
	encrypted, err := cm.encryptionManager.Encrypt(data)
	if err != nil {
		return fmt.Errorf("failed to encrypt credentials: %w", err)
	}

	if err := os.WriteFile(cm.credentialsFile, encrypted, 0600); err != nil {
		return fmt.Errorf("failed to write credentials file: %w", err)
	}

	return nil
}
