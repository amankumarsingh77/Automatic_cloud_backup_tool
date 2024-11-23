package backup

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/robfig/cron/v3"

	filesync "github.com/amankumarsingh77/automated_backup_tool/internal/core/sync"
	"github.com/amankumarsingh77/automated_backup_tool/internal/security/credentials"
	"github.com/amankumarsingh77/automated_backup_tool/internal/security/encryption"
	"github.com/amankumarsingh77/automated_backup_tool/internal/storage/gdrive"
	"github.com/amankumarsingh77/automated_backup_tool/internal/storage/onedrive"
	"github.com/amankumarsingh77/automated_backup_tool/internal/utils"
	"github.com/amankumarsingh77/automated_backup_tool/internal/utils/filesystem"
	"github.com/amankumarsingh77/automated_backup_tool/internal/utils/retry"
	"github.com/google/uuid"
	"github.com/madflojo/tasks"
)

const (
	StatusPending   = "pending"
	StatusRunning   = "running"
	StatusCompleted = "completed"
	StatusFailed    = "failed"
	StatusSyncing   = "syncing"
)

type BackupTask struct {
	ID              string    `json:"id"`
	SourcePath      string    `json:"source_path"`
	Provider        string    `json:"provider"`
	DestinationPath string    `json:"destination_path"`
	Schedule        string    `json:"schedule"`
	Recurring       bool      `json:"recurring"`
	Compress        bool      `json:"compress"`
	Encrypt         bool      `json:"encrypt"`
	EncryptionKey   string    `json:"encryption_key,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	Status          string    `json:"status"`
	IsSingle        bool      `json:"is_single"`
	IsSync          bool      `json:"is_sync"`
	ErrorMessage    string    `json:"error_message,omitempty"`
	watcher         *filesync.FolderWatcher
	stopSync        chan struct{}
}

type TaskManager struct {
	mu          sync.RWMutex
	schedulers  map[string]*cron.Cron
	credManager *credentials.CredentialManager
}

func (tm *TaskManager) Initialize(masterPassword string) error {
	var err error
	tm.mu.Lock()
	defer tm.mu.Unlock()

	
	tm.credManager, err = credentials.NewCredentialManager(masterPassword)
	if err != nil {
		return fmt.Errorf("failed to initialize credential manager: %w", err)
	}

	
	tm.schedulers = make(map[string]*cron.Cron)
	return nil
}

func (t *BackupTask) Create() (string, error) {
	t.ID = uuid.New().String()
	t.CreatedAt = time.Now()
	t.Status = StatusPending

	if t.SourcePath == "" || t.Provider == "" {
		return "", errors.New("cannot create a backup task without a Sourcepath or Provider")
	}

	totTasks, err := LoadTasks()
	if err != nil {
		return "", err
	}

	totTasks = append(totTasks, *t)

	err = SaveTasks(totTasks)
	if err != nil {
		return "", err
	}

	return t.ID, nil
}

func ListTasks() ([]BackupTask, error) {
	totTasks, err := LoadTasks()
	if err != nil {
		return nil, err
	}
	return totTasks, nil
}

func (t *BackupTask) DeleteTask() error {
	totTasks, err := LoadTasks()
	if err != nil {
		return err
	}

	var updatedTasks []BackupTask
	for _, task := range totTasks {
		if task.ID != t.ID {
			updatedTasks = append(updatedTasks, task)
		}
	}

	return SaveTasks(updatedTasks)
}

func (t *BackupTask) ExecuteTask() error {
	logger := utils.GetLogger()
	logger.Info("Starting backup task %s", t.ID)

	if t.IsSync {
		return t.startSync()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
	defer cancel()

	
	backoff := retry.NewExponentialBackoff().
		WithInitialDelay(5 * time.Second).
		WithMaxDelay(1 * time.Minute)

	
	operation := func() error {
		t.Status = StatusRunning
		if err := UpdateTaskStatus(t.ID, t.Status, ""); err != nil {
			logger.Error("Failed to update task status: %v", err)
			return retry.NewRetryableError(err, true)
		}

		
		defer func() {
			if r := recover(); r != nil {
				errMsg := fmt.Sprintf("task panic: %v", r)
				logger.Error("Task panic: %v", r)
				t.Status = StatusFailed
				UpdateTaskStatus(t.ID, t.Status, errMsg)
			}
		}()

		filePath := t.SourcePath
		var err error

		
		if t.Encrypt {
			logger.Info("Encrypting file %s", filePath)
			if t.EncryptionKey == "" {
				logger.Info("No encryption key provided, generating random key")
				t.EncryptionKey, err = encryption.GenerateRandomKey()
				if err != nil {
					logger.Error("Failed to generate encryption key: %v", err)
					return retry.NewRetryableError(err, false)
				}
			}

			encryptionManager, err := encryption.NewEncryptionManager(t.EncryptionKey)
			if err != nil {
				logger.Error("Failed to create encryption manager: %v", err)
				return retry.NewRetryableError(err, false)
			}

			filePath, err = encryptionManager.EncryptFile(filePath)
			if err != nil {
				logger.Error("Failed to encrypt file: %v", err)
				return retry.NewRetryableError(err, false)
			}
			defer os.Remove(filePath) 
			logger.Info("File encrypted successfully")
		}

		
		if t.Compress {
			logger.Info("Compressing file %s", filePath)
			filePath, err = filesystem.CompressFile(filePath)
			if err != nil {
				errMsg := fmt.Sprintf("could not compress backup task: %s", err)
				logger.Error("Compression failed: %v", err)
				t.Status = StatusFailed
				UpdateTaskStatus(t.ID, t.Status, errMsg)
				return retry.NewRetryableError(err, true)
			}
			defer os.Remove(filePath) 
			logger.Info("File compressed successfully")
		}

		logger.Info("Getting credentials for provider %s", t.Provider)
		creds, err := GlobalTaskManager.credManager.GetCredential(t.Provider)
		if err != nil {
			logger.Error("Failed to get credentials: %v", err)
			return retry.NewRetryableError(err, false)
		}

		switch t.Provider {
		case "gdrive":
			logger.Info("Uploading to Google Drive: %s", t.DestinationPath)
			gdriveProvider := gdrive.NewGoogleDriveProvider()
			gdriveProvider.SetCredentials(creds.Key, creds.Secret, creds.RedirectURL)
			err = gdriveProvider.Upload(t.IsSingle, filePath, t.DestinationPath)
			if err != nil {
				errMsg := fmt.Sprintf("cannot upload backup task to gdrive: %v", err)
				logger.Error("Google Drive upload failed: %v", err)
				t.Status = StatusFailed
				UpdateTaskStatus(t.ID, t.Status, errMsg)
				return retry.NewRetryableError(err, true)
			}
			logger.Info("Successfully uploaded to Google Drive")

		case "onedrive":
			logger.Info("Uploading to OneDrive: %s", t.DestinationPath)
			onedriveProvider := onedrive.NewOneDriveProvider()
			err = onedriveProvider.Upload(filePath, t.DestinationPath)
			if err != nil {
				errMsg := fmt.Sprintf("cannot upload backup task to onedrive: %v", err.Error())
				logger.Error("OneDrive upload failed: %v", err)
				t.Status = StatusFailed
				UpdateTaskStatus(t.ID, t.Status, errMsg)
				return retry.NewRetryableError(err, true)
			}
			logger.Info("Successfully uploaded to OneDrive")

		default:
			errMsg := fmt.Sprintf("unsupported provider %s", t.Provider)
			logger.Error("Unsupported provider: %s", t.Provider)
			t.Status = StatusFailed
			UpdateTaskStatus(t.ID, t.Status, errMsg)
			return retry.NewRetryableError(fmt.Errorf(errMsg), false)
		}

		t.Status = StatusCompleted
		if err := UpdateTaskStatus(t.ID, t.Status, ""); err != nil {
			logger.Error("Failed to update task status: %v", err)
			return retry.NewRetryableError(err, true)
		}

		logger.Info("Backup task %s completed successfully", t.ID)
		return nil
	}

	
	return backoff.RetryWithBackoff(ctx, operation)
}

func (t *BackupTask) startSync() error {
	logger := utils.GetLogger()
	logger.Info("Starting sync task for folder: %s", t.SourcePath)

	
	stateFile := filepath.Join(os.TempDir(), fmt.Sprintf("sync_state_%s.json", t.ID))

	watcher, err := filesync.NewFolderWatcher(stateFile)
	if err != nil {
		return fmt.Errorf("failed to create folder watcher: %v", err)
	}

	t.watcher = watcher
	t.stopSync = make(chan struct{})
	t.Status = StatusSyncing

	
	if err := watcher.WatchFolder(t.SourcePath); err != nil {
		return fmt.Errorf("failed to start watching folder: %v", err)
	}

	
	go func() {
		uploadChan := watcher.GetUploadChannel()
		for {
			select {
			case filePath := <-uploadChan:
				
				relPath, err := filepath.Rel(t.SourcePath, filePath)
				if err != nil {
					logger.Error("Failed to get relative path: %v", err)
					continue
				}

				
				fileTask := &BackupTask{
					ID:              t.ID,
					SourcePath:      filePath,
					Provider:        t.Provider,
					DestinationPath: filepath.Join(t.DestinationPath, relPath),
					Encrypt:         t.Encrypt,
					EncryptionKey:   t.EncryptionKey,
					Compress:        t.Compress,
					IsSingle:        true,
					Status:          StatusPending,
				}

				
				if err := fileTask.ExecuteTask(); err != nil {
					logger.Error("Failed to upload file %s: %v", filePath, err)
				} else {
					logger.Info("Successfully synced file: %s", filePath)
				}

			case <-t.stopSync:
				watcher.Stop()
				t.Status = StatusCompleted
				return
			}
		}
	}()

	
	syncDone := make(chan struct{})
	go func() {
		<-t.stopSync
		close(syncDone)
	}()
	<-syncDone

	return nil
}

func (t *BackupTask) StopSync() error {
	if t.IsSync && t.stopSync != nil {
		close(t.stopSync)
		t.Status = StatusCompleted
	}
	return nil
}

func (t *BackupTask) ScheduleTask() error {
	scheduler := tasks.New()
	done := make(chan bool)
	errChan := make(chan error)

	if t.Recurring {
		duration, err := utils.CronToDuration(t.Schedule)
		if err != nil {
			return fmt.Errorf("cannot schedule backup task %s: %v", t.ID, err)
		}

		_, err = scheduler.Add(&tasks.Task{
			Interval: duration,
			TaskFunc: func() error {
				if err := t.ExecuteTask(); err != nil {
					log.Printf("Error in recurring task %s: %v", t.ID, err)
					errChan <- err
					return err
				}
				return nil
			},
		})
		if err != nil {
			return fmt.Errorf("cannot schedule recurring backup task %s: %s", t.ID, err)
		}

		
		select {
		case err := <-errChan:
			scheduler.Stop()
			return fmt.Errorf("recurring backup task %s failed: %v", t.ID, err)
		}

	} else {
		_, err := scheduler.Add(&tasks.Task{
			Interval: time.Second * 1,
			RunOnce:  true,
			TaskFunc: func() error {
				if err := t.ExecuteTask(); err != nil {
					errChan <- err
					return err
				}
				done <- true
				return nil
			},
		})
		if err != nil {
			return fmt.Errorf("cannot schedule one-time backup task %s: %s", t.ID, err)
		}

		
		select {
		case <-done:
			scheduler.Stop()
			return nil
		case err := <-errChan:
			scheduler.Stop()
			return fmt.Errorf("backup task %s failed: %v", t.ID, err)
		case <-time.After(30 * time.Minute): 
			scheduler.Stop()
			return fmt.Errorf("backup task %s timed out", t.ID)
		}
	}
	
	return nil
}

func (t *BackupTask) TempSchedule() error {
	scheduler := cron.New()
	var task func()
	if t.Recurring {
		task = func() {
			fmt.Printf("Executing task: %v\n", t.ID)
			if err := t.ExecuteTask(); err != nil {
				log.Printf("Backup task %s failed: %v", t.ID, err)
			}
		}
	} else {
		task = func() {
			fmt.Printf("Executing task: %v\n", t.ID)
			if err := t.ExecuteTask(); err != nil {
				log.Printf("Backup task %s failed: %v", t.ID, err)
			}
			scheduler.Stop()
		}
	}

	_, err := scheduler.AddFunc(t.Schedule, task)
	if err != nil {
		return fmt.Errorf("could not schedule backup task %s: %v", t.ID, err.Error())
	}
	scheduler.Start()
	return nil
}

func (tm *TaskManager) AddTask(task *BackupTask) error {
	scheduler := cron.New()

	taskFunc := func() {
		fmt.Printf("Executing task: %v\n", task.ID)
		if err := task.ExecuteTask(); err != nil {
			log.Printf("Failed to start backup task %s: %v", task.ID, err)
		}

		if !task.Recurring {
			tm.RemoveTask(task.ID)
		}
	}

	_, err := scheduler.AddFunc(task.Schedule, taskFunc)
	if err != nil {
		return fmt.Errorf("could not schedule backup task %s: %v", task.ID, err.Error())
	}

	scheduler.Start()
	tm.schedulers[task.ID] = scheduler
	return nil
}

func (tm *TaskManager) RemoveTask(taskID string) {
	if scheduler, exists := tm.schedulers[taskID]; exists {
		scheduler.Stop()
		delete(tm.schedulers, taskID)
	}
}

func (tm *TaskManager) StopAllTasks() {
	for _, scheduler := range tm.schedulers {
		scheduler.Stop()
	}
	tm.schedulers = make(map[string]*cron.Cron)
}

var GlobalTaskManager = &TaskManager{
	schedulers: make(map[string]*cron.Cron),
}
