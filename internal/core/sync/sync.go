package sync

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

type FileInfo struct {
	Path         string    `json:"path"`
	Hash         string    `json:"hash"`
	LastModified time.Time `json:"last_modified"`
	Size         int64     `json:"size"`
}

type FolderState struct {
	mu        sync.RWMutex
	Files     map[string]FileInfo `json:"files"`
	StateFile string
}

func NewFolderState(stateFile string) *FolderState {
	return &FolderState{
		Files:     make(map[string]FileInfo),
		StateFile: stateFile,
	}
}

func (fs *FolderState) calculateFileHash(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

func (fs *FolderState) UpdateFile(path string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	fileInfo, err := os.Stat(path)
	if err != nil {
		return err
	}

	if fileInfo.IsDir() {
		return nil
	}

	hash, err := fs.calculateFileHash(path)
	if err != nil {
		return err
	}

	fs.Files[path] = FileInfo{
		Path:         path,
		Hash:         hash,
		LastModified: fileInfo.ModTime(),
		Size:         fileInfo.Size(),
	}

	return nil
}

func (fs *FolderState) HasFileChanged(path string) (bool, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	oldInfo, exists := fs.Files[path]
	if !exists {
		return true, nil
	}

	fileInfo, err := os.Stat(path)
	if err != nil {
		return false, err
	}

	if fileInfo.ModTime().After(oldInfo.LastModified) {
		hash, err := fs.calculateFileHash(path)
		if err != nil {
			return false, err
		}
		return hash != oldInfo.Hash, nil
	}

	return false, nil
}

func (fs *FolderState) SaveState() error {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	data, err := json.MarshalIndent(fs.Files, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %v", err)
	}

	return os.WriteFile(fs.StateFile, data, 0644)
}

func (fs *FolderState) LoadState() error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	data, err := os.ReadFile(fs.StateFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil 
		}
		return fmt.Errorf("failed to read state file: %v", err)
	}

	return json.Unmarshal(data, &fs.Files)
}

type FolderWatcher struct {
	watcher    *fsnotify.Watcher
	state      *FolderState
	uploadChan chan string
	done       chan struct{}
}

func NewFolderWatcher(stateFile string) (*FolderWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	return &FolderWatcher{
		watcher:    watcher,
		state:      NewFolderState(stateFile),
		uploadChan: make(chan string, 100),
		done:       make(chan struct{}),
	}, nil
}

func (fw *FolderWatcher) WatchFolder(folderPath string) error {
	
	if err := fw.state.LoadState(); err != nil {
		return fmt.Errorf("failed to load previous state: %v", err)
	}

	
	err := filepath.Walk(folderPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return fw.watcher.Add(path)
		}

		
		changed, err := fw.state.HasFileChanged(path)
		if err != nil {
			return err
		}

		if changed {
			fw.uploadChan <- path
		}

		return fw.state.UpdateFile(path)
	})

	if err != nil {
		return fmt.Errorf("error walking folder: %v", err)
	}

	go fw.watchEvents()
	return nil
}

func (fw *FolderWatcher) watchEvents() {
	for {
		select {
		case event, ok := <-fw.watcher.Events:
			if !ok {
				return
			}

			if event.Op&(fsnotify.Write|fsnotify.Create) != 0 {
				fileInfo, err := os.Stat(event.Name)
				if err != nil {
					continue
				}

				if fileInfo.IsDir() {
					fw.watcher.Add(event.Name)
					continue
				}

				changed, err := fw.state.HasFileChanged(event.Name)
				if err != nil {
					continue
				}

				if changed {
					fw.uploadChan <- event.Name
					fw.state.UpdateFile(event.Name)
				}
			}

		case err, ok := <-fw.watcher.Errors:
			if !ok {
				return
			}
			
			fmt.Printf("Watcher error: %v\n", err)

		case <-fw.done:
			return
		}
	}
}

func (fw *FolderWatcher) Stop() {
	close(fw.done)
	fw.watcher.Close()
	
	
	if err := fw.state.SaveState(); err != nil {
		fmt.Printf("Error saving sync state: %v\n", err)
	}
}

func (fw *FolderWatcher) GetUploadChannel() <-chan string {
	return fw.uploadChan
}
