package main

import (
	onedrive2 "github.com/amankumarsingh77/automated_backup_tool/internal/storage/onedrive"
	"log"
)

func main() {
	onedrive := onedrive2.NewOneDriveProvider()
	err := onedrive.Authenticate()
	if err != nil {
		log.Fatalf("Failed to authenticate: %v", err)
	}

	//dir, err := os.Getwd()
	//if err != nil {
	//	log.Fatalf("Failed to get working directory: %v", err)
	//}

	//demoFilesDir := filepath.Join(dir, "demo_files")
	//
	//err = onedrive.Download(demoFilesDir, "EcNjzoYJTHVBuI-XB6lxImkBDMYWfATP_SS7uP3OBTlJ1A")
	//if err != nil {
	//	log.Fatalf("Failed to download: %v", err)
	//}

	files, err := onedrive.ListFiles("01555CD3EJG56MA2YZQREY7G4BGVNTTZYC")
	if err != nil {
		log.Fatalf("Failed to list files: %v", err)
	}
	log.Println(files[2].ID)

	//err = filepath.Walk(demoFilesDir, func(path string, info os.FileInfo, err error) error {
	//	if err != nil {
	//		return fmt.Errorf("error accessing path %q: %v", path, err)
	//	}
	//	if info.IsDir() {
	//		return nil
	//	}
	//	fmt.Printf("Uploading file: %s\n", path)
	//
	//	err = onedrive.Upload(path, "/auto_backup/main")
	//	if err != nil {
	//		return fmt.Errorf("failed to upload file %q: %v", path, err)
	//	}
	//
	//	fmt.Printf("Successfully uploaded file: %s\n", path)
	//	return nil
	//})
	//
	//if err != nil {
	//	log.Fatalf("Error while walking through demo_files directory: %v", err)
	//}
	//
	//fmt.Println("All files uploaded successfully.")

}
