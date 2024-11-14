package backup

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

var dir, _ = os.Getwd()

var taskFile = filepath.Join(dir, "backup_tasks.json")

func LoadTasks() ([]BackupTask, error) {
	file, err := os.ReadFile(taskFile)
	if err != nil {
		if os.IsNotExist(err) {
			return []BackupTask{}, nil
		}
		return nil, err
	}

	if len(file) == 0 {
		return []BackupTask{}, nil
	}

	var tasks []BackupTask
	if err := json.Unmarshal(file, &tasks); err != nil {
		return nil, err
	}
	return tasks, nil
}

func SaveTasks(tasks []BackupTask) error {
	data, err := json.MarshalIndent(tasks, "", "\t")
	if err != nil {
		return err
	}
	return os.WriteFile(taskFile, data, 0644)
}

func UpdateTask(task BackupTask) error {
	tasks, err := LoadTasks()
	if err != nil {
		return fmt.Errorf("failed to load tasks: %w", err)
	}

	taskFound := false
	for i, t := range tasks {
		if t.ID == task.ID {
			tasks[i] = task
			taskFound = true
			break
		}
	}

	if !taskFound {
		return fmt.Errorf("task with ID %s not found", task.ID)
	}

	if err := SaveTasks(tasks); err != nil {
		return fmt.Errorf("failed to save tasks: %w", err)
	}

	return nil
}

func UpdateTaskStatus(taskID, status, errorMessage string) error {
	tasks, err := LoadTasks()
	if err != nil {
		return fmt.Errorf("failed to load tasks: %w", err)
	}

	taskFound := false
	for i, task := range tasks {
		if task.ID == taskID {
			tasks[i].Status = status
			tasks[i].ErrorMessage = errorMessage
			taskFound = true
			break
		}
	}

	if !taskFound {
		return fmt.Errorf("task with ID %s not found", taskID)
	}

	if err := SaveTasks(tasks); err != nil {
		return fmt.Errorf("failed to save tasks: %w", err)
	}

	if errorMessage != "" {
		log.Printf("Task %s status updated to %s with error: %s", taskID, status, errorMessage)
	} else {
		log.Printf("Task %s status updated to %s", taskID, status)
	}

	return nil
}
