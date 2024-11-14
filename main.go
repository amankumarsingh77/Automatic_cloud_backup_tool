package main

import (
	"fmt"
	"github.com/amankumarsingh77/automated_backup_tool/internal/core/backup"
	"log"
)

func main() {
	task := &backup.BackupTask{
		SourcePath:      `C:\Users\PC\Downloads\Biig FRESH proxy list 01-11-24 ( HTTPS ).txt`,
		Provider:        "onedrive",
		DestinationPath: "/aman_demo_automated",
		Recurring:       false,
		Compress:        false,
		Encrypt:         false,
	}

	taskId, err := task.Create()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(taskId)

	err = task.ScheduleTask()
	if err != nil {
		log.Fatal(err)
	}
}
