package storage

type StorageProvider interface {
	Authenticate() error
	Upload(localPath, remotePath string) error
	Download(localPath, remotePath string) error
	ListFiles(remotePath string) ([]string, error) //TODO: Add File Struct
}
