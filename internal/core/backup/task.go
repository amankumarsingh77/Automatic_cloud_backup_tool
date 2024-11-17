package backup

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/amankumarsingh77/automated_backup_tool/internal/storage/gdrive"
	"github.com/amankumarsingh77/automated_backup_tool/internal/storage/onedrive"
	"github.com/amankumarsingh77/automated_backup_tool/internal/utils"
	"github.com/amankumarsingh77/automated_backup_tool/internal/utils/filesystem"
	"github.com/google/uuid"
	"github.com/madflojo/tasks"
)

const (
	StatusPending   = "pending"
	StatusRunning   = "running"
	StatusCompleted = "completed"
	StatusFailed    = "failed"
)

type BackupTask struct {
	ID              string
	SourcePath      string
	Provider        string
	DestinationPath string
	Schedule        string
	Recurring       bool
	Compress        bool
	Encrypt         bool
	Status          string
	CreatedAt       time.Time
	ErrorMessage    string
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
	t.Status = StatusRunning
	if err := UpdateTaskStatus(t.ID, t.Status, ""); err != nil {
		log.Printf("Failed to update task status: %v", err)
	}

	// Ensure status gets updated on panic
	defer func() {
		if r := recover(); r != nil {
			errMsg := fmt.Sprintf("task panic: %v", r)
			t.Status = StatusFailed
			UpdateTaskStatus(t.ID, t.Status, errMsg)
		}
	}()

	filePath := t.SourcePath
	var err error

	if t.Compress {
		filePath, err = filesystem.CompressFile(filePath)
		if err != nil {
			errMsg := fmt.Sprintf("could not compress backup task: %s", err)
			t.Status = StatusFailed
			UpdateTaskStatus(t.ID, t.Status, errMsg)
			return fmt.Errorf(errMsg)
		}
	}

	switch t.Provider {
	case "gdrive":
		gdriveProvider := gdrive.NewGoogleDriveProvider()
		err := gdriveProvider.Upload(filePath, t.DestinationPath)
		if err != nil {
			errMsg := fmt.Sprintf("cannot upload backup task to gdrive: %v", err)
			t.Status = StatusFailed
			UpdateTaskStatus(t.ID, t.Status, errMsg)
			return fmt.Errorf(errMsg)
		}
	case "onedrive":
		onedriveProvider := onedrive.NewOneDriveProvider()
		err := onedriveProvider.Upload(filePath, t.DestinationPath)
		if err != nil {
			errMsg := fmt.Sprintf("cannot upload backup task to onedrive: %v", err.Error())
			t.Status = StatusFailed
			UpdateTaskStatus(t.ID, t.Status, errMsg)
			return fmt.Errorf(errMsg)
		}
	default:
		errMsg := fmt.Sprintf("unsupported provider %s", t.Provider)
		t.Status = StatusFailed
		UpdateTaskStatus(t.ID, t.Status, errMsg)
		return fmt.Errorf(errMsg)
	}

	t.Status = StatusCompleted
	if err := UpdateTaskStatus(t.ID, t.Status, ""); err != nil {
		log.Printf("Failed to update final task status: %v", err)
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

		// For recurring tasks, we keep the scheduler running
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

		// Wait for either completion or error
		select {
		case <-done:
			scheduler.Stop()
			return nil
		case err := <-errChan:
			scheduler.Stop()
			return fmt.Errorf("backup task %s failed: %v", t.ID, err)
		case <-time.After(30 * time.Minute): // Increased timeout for large files
			scheduler.Stop()
			return fmt.Errorf("backup task %s timed out", t.ID)
		}
	}
	return nil
}
