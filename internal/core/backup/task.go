package backup

import (
	"errors"
	"fmt"
	"github.com/amankumarsingh77/automated_backup_tool/internal/storage/gdrive"
	"github.com/amankumarsingh77/automated_backup_tool/internal/storage/onedrive"
	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
	"log"
	"time"
)

type BackupTask struct {
	ID              string
	SourcePath      string
	Provider        string
	DestinationPath string
	Schedule        string
	Compress        bool
	Encrypt         bool
	CreatedAt       time.Time
}

func (t *BackupTask) Create() (string, error) {
	t.ID = uuid.New().String()
	if t.SourcePath == "" || t.Provider == "" {
		return "", errors.New("cannot create a backup task without a Sourcepath or Provider")
	}
	tasks, err := LoadTasks()
	if err != nil {
		return "", err
	}
	tasks = append(tasks, *t)
	err = SaveTasks(tasks)
	if err != nil {
		return "", err
	}
	return t.ID, nil
}

//TODO still deciding on which approach to choose
//func CreateTask(sourcePath, provider, schedule string, compress, encrypt bool) (string, error) {
//	task := BackupTask{
//		ID:         uuid.NewString(),
//		SourcePath: sourcePath,
//		Provider:   provider,
//		Schedule:   schedule,
//		Compress:   compress,
//		Encrypt:    encrypt,
//		CreatedAt:  time.Now(),
//	}
//	tasks, err := LoadTasks()
//	if err != nil {
//		return task, err
//	}
//	tasks = append(tasks, task)
//	if err := SaveTasks(tasks); err != nil {
//		return task, err
//	}
//	return task, nil
//}

func ListTasks() ([]BackupTask, error) {
	tasks, err := LoadTasks()
	if err != nil {
		return nil, err
	}
	return tasks, nil
}

func (t *BackupTask) DeleteTask() error {
	tasks, err := LoadTasks()
	if err != nil {
		return err
	}
	var updatedTasks []BackupTask
	for _, task := range tasks {
		if task.ID != t.ID {
			updatedTasks = append(updatedTasks, task)
		}
	}
	return SaveTasks(updatedTasks)
}

func (t *BackupTask) ExecuteTask() error {
	filePath := t.SourcePath
	if t.Compress {
		filePath = utils.CompressFile(filePath)
	}
	if t.Encrypt {
		filePath = utils.EncryptFile(filePath)
	}

	switch t.Provider {
	case "gdrive":
		gdriveProvider := gdrive.NewGoogleDriveProvider()
		err := gdriveProvider.Upload(t.SourcePath, t.DestinationPath)
		if err != nil {
			return fmt.Errorf("cannot upload backup task to gdrive: %v", err)
		}
	case "onedrive":
		onedriveProvider := onedrive.NewOneDriveProvider()
		err := onedriveProvider.Upload(t.SourcePath, t.DestinationPath)
		if err != nil {
			return fmt.Errorf("cannot upload backup task to onedrive: %v", err)
		}
	}
	//TODO Implement logging of task completion
	//LogTaskCompletion(t.ID)
	return nil
}

func (t *BackupTask) ScheduleTask() error {
	cronScheduler := cron.New()
	cronScheduler.AddFunc(t.Schedule, func() {
		err := t.ExecuteTask()
		if err != nil {
			log.Printf("cannot schedule backup task: %v", err)
		}
	})
	cronScheduler.Start()
	return nil
}
