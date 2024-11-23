package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"os/signal"

	"github.com/amankumarsingh77/automated_backup_tool/internal/core/backup"
	"github.com/amankumarsingh77/automated_backup_tool/internal/security/credentials"
	"github.com/google/uuid"
)

var masterPassword string

func init() {
	flag.StringVar(&masterPassword, "master-password", "", "Master password for credential encryption")
}

func main() {
	flag.Parse()
	args := flag.Args()

	if masterPassword == "" {
		log.Fatal("Master password is required. Use -master-password flag")
	}

	if err := backup.GlobalTaskManager.Initialize(masterPassword); err != nil {
		log.Fatalf("Failed to initialize task manager: %v", err)
	}

	createCmd := flag.NewFlagSet("create", flag.ExitOnError)
	listCmd := flag.NewFlagSet("list", flag.ExitOnError)
	configureCmd := flag.NewFlagSet("configure", flag.ExitOnError)

	sourcePath := createCmd.String("source", "", "Source path to backup")
	provider := createCmd.String("provider", "gdrive", "Cloud provider (gdrive or onedrive)")
	destPath := createCmd.String("dest", "", "Destination path in cloud storage")
	schedule := createCmd.String("schedule", "", "Backup schedule in cron format (optional)")
	recurring := createCmd.Bool("recurring", false, "Whether the backup should recur")
	compress := createCmd.Bool("compress", false, "Whether to compress the backup")
	encrypt := createCmd.Bool("encrypt", false, "Whether to encrypt the backup")
	encryptKey := createCmd.String("key", "", "Encryption key (required if encrypt is true)")
	isSingle := createCmd.Bool("single", false, "Whether to backup single file")
	isSync := createCmd.Bool("sync", false, "Whether to enable folder synchronization")

	configProvider := configureCmd.String("provider", "gdrive", "Provider to configure (gdrive or onedrive)")
	clientID := configureCmd.String("client-id", "", "OAuth client ID")
	clientSecret := configureCmd.String("client-secret", "", "OAuth client secret")
	redirectURL := configureCmd.String("redirect-url", "http://localhost:8080/callback", "OAuth redirect URL")

	if len(args) < 1 {
		printUsage()
		os.Exit(1)
	}

	switch args[0] {
	case "create":
		createCmd.Parse(args[1:])
		handleCreate(*sourcePath, *provider, *destPath, *schedule, *recurring, *compress, *encrypt, *encryptKey, *isSingle, *isSync)
	case "list":
		listCmd.Parse(args[1:])
		handleList()
	case "configure":
		configureCmd.Parse(args[1:])
		handleConfigure(*configProvider, *clientID, *clientSecret, *redirectURL)
	default:
		printUsage()
		os.Exit(1)
	}
}

func handleCreate(sourcePath, provider, destPath, schedule string, recurring, compress, encrypt bool, encryptKey string, isSingle, isSync bool) {
	if sourcePath == "" || destPath == "" {
		log.Fatal("Source path and destination path are required")
	}

	absPath, err := filepath.Abs(sourcePath)
	if err != nil {
		log.Fatalf("Error getting absolute path: %v", err)
	}

	task := &backup.BackupTask{
		ID:              uuid.New().String(),
		SourcePath:      absPath,
		Provider:        provider,
		DestinationPath: destPath,
		Schedule:        schedule,
		Recurring:       recurring,
		Compress:        compress,
		Encrypt:         encrypt,
		EncryptionKey:   encryptKey,
		CreatedAt:       time.Now(),
		Status:          backup.StatusPending,
		IsSingle:        isSingle,
		IsSync:          isSync,
	}

	if encrypt && encryptKey == "" {
		log.Fatal("Encryption key is required when encryption is enabled")
	}

	if _, err := task.Create(); err != nil {
		log.Fatalf("Failed to create task: %v", err)
	}

	if schedule != "" {
		if err := task.ScheduleTask(); err != nil {
			log.Fatalf("Failed to schedule task: %v", err)
		}
		fmt.Printf("Task scheduled successfully with ID: %s\n", task.ID)
		return
	}

	if err := task.ExecuteTask(); err != nil {
		log.Fatalf("Failed to execute task: %v", err)
	}

	fmt.Printf("Task started with ID: %s\n", task.ID)

	// If this is a sync task, keep the program running
	if isSync {
		fmt.Println("Folder sync is active. Press Ctrl+C to stop...")

		// Set up signal handling
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

		// Wait for interrupt signal
		<-sigChan
		fmt.Println("\nReceived interrupt signal. Stopping sync...")

		// Stop the sync task gracefully
		if err := task.StopSync(); err != nil {
			log.Printf("Error stopping sync: %v", err)
		}
		fmt.Println("Sync stopped successfully")
	} else {
		fmt.Printf("Task completed successfully with ID: %s\n", task.ID)
	}
}

func handleList() {
	tasks, err := backup.ListTasks()
	if err != nil {
		log.Fatalf("Failed to list tasks: %v", err)
	}

	if len(tasks) == 0 {
		fmt.Println("No backup tasks found")
		return
	}

	fmt.Println("\nBackup Tasks:")
	fmt.Println("-------------")
	for _, task := range tasks {
		fmt.Printf("ID: %s\n", task.ID)
		fmt.Printf("Source: %s\n", task.SourcePath)
		fmt.Printf("Provider: %s\n", task.Provider)
		fmt.Printf("Destination: %s\n", task.DestinationPath)
		fmt.Printf("Status: %s\n", task.Status)
		if task.Schedule != "" {
			fmt.Printf("Schedule: %s\n", task.Schedule)
		}
		if task.ErrorMessage != "" {
			fmt.Printf("Error: %s\n", task.ErrorMessage)
		}
		fmt.Println("-------------")
	}
}

func handleConfigure(provider, clientID, clientSecret, redirectURL string) {
	if clientID == "" || clientSecret == "" {
		log.Fatal("Client ID and Client Secret are required")
	}

	credManager, err := credentials.NewCredentialManager(masterPassword)
	if err != nil {
		log.Fatalf("Failed to initialize credential manager: %v", err)
	}

	cred := credentials.Credential{
		Provider:    provider,
		Key:         clientID,
		Secret:      clientSecret,
		RedirectURL: redirectURL,
	}

	if err := credManager.StoreCredential(cred); err != nil {
		log.Fatalf("Failed to store credentials: %v", err)
	}

	fmt.Printf("Successfully configured credentials for %s\n", provider)
	fmt.Println("\nYou can now create backup tasks using this provider.")
}

func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  backup-service create [flags]")
	fmt.Println("  backup-service list")
	fmt.Println("  backup-service configure [flags]")
	fmt.Println("\nCreate flags:")
	fmt.Println("  -source    Source path to backup")
	fmt.Println("  -provider  Cloud provider (gdrive or onedrive)")
	fmt.Println("  -dest      Destination path in cloud storage")
	fmt.Println("  -schedule  Backup schedule in cron format (optional)")
	fmt.Println("  -recurring Enable recurring backup")
	fmt.Println("  -compress  Enable compression (default: true)")
	fmt.Println("  -encrypt   Enable encryption")
	fmt.Println("  -key       Encryption key (required if encrypt is true)")
	fmt.Println("  -single    Single file backup")
	fmt.Println("  -sync      Enable folder synchronization")
	fmt.Println("\nConfigure flags:")
	fmt.Println("  -provider  Provider to configure (gdrive or onedrive)")
	fmt.Println("  -client-id OAuth client ID")
	fmt.Println("  -client-secret OAuth client secret")
	fmt.Println("  -redirect-url OAuth redirect URL (default: http://localhost:8080/callback)")
	fmt.Println("\nExamples:")
	fmt.Println("  backup-service create -source /path/to/backup -provider gdrive -dest /backups")
	fmt.Println("  backup-service create -source /path/to/backup -provider gdrive -dest /backups -schedule \"0 0 * * *\" -recurring")
	fmt.Println("  backup-service list")
	fmt.Println("  backup-service configure -provider gdrive -client-id <client-id> -client-secret <client-secret>")
}
