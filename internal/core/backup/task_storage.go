package backup

import (
	"encoding/json"
	"os"
)

const taskFile = "backup_tasks.json"

func LoadTasks() ([]BackupTask, error) {
	file, err := os.ReadFile(taskFile)
	if err != nil {
		if os.IsExist(err) {
			return []BackupTask{}, err
		}
		return nil, err
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
