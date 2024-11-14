package gdrive

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/amankumarsingh77/automated_backup_tool/internal/utils/filesystem"
	"github.com/joho/godotenv"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

type GoogleDriveProvider struct {
	service         *drive.Service
	token           *oauth2.Token
	isAuthenticated bool
}

func NewGoogleDriveProvider() *GoogleDriveProvider {
	provider := &GoogleDriveProvider{}
	if err := provider.Authenticate(); err != nil {
		log.Printf("Failed to authenticate Google Drive provider: %v", err)
	}
	return provider
}

func init() {
	if err := godotenv.Load(".env"); err != nil {
		log.Printf("Warning: Error loading .env file: %v", err)
	}
}

func (p *GoogleDriveProvider) Authenticate() error {
	if p.isAuthenticated && p.service != nil {
		return nil
	}

	config := oauth2.Config{
		ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),

		Endpoint:    google.Endpoint,
		RedirectURL: os.Getenv("GOOGLE_CLIENT_REDIRECT_URL"),
		Scopes:      []string{drive.DriveScope},
	}

	token, err := filesystem.GetToken("google-drive")
	if err == nil {
		p.token = token
		client := config.Client(context.Background(), p.token)
		if p.service, err = drive.NewService(context.Background(), option.WithHTTPClient(client)); err != nil {
			return fmt.Errorf("failed to create drive service: %w", err)
		}
		p.isAuthenticated = true
		return nil
	}

	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Visit the following URL to authenticate: %s\n", authURL)

	var code string
	fmt.Print("Enter the authorization code: ")
	if _, err := fmt.Scanln(&code); err != nil {
		return fmt.Errorf("failed to read authorization code: %w", err)
	}

	token, err = config.Exchange(context.Background(), code)
	if err != nil {
		return fmt.Errorf("failed to exchange token: %w", err)
	}

	p.token = token
	client := config.Client(context.Background(), p.token)

	if p.service, err = drive.NewService(context.Background(), option.WithHTTPClient(client)); err != nil {
		return fmt.Errorf("failed to create drive service: %w", err)
	}

	if err = filesystem.SaveTokenLocally("google-drive", p.token); err != nil {
		return fmt.Errorf("failed to save token locally: %w", err)
	}

	p.isAuthenticated = true
	return nil
}

func (p *GoogleDriveProvider) Upload(localPath, remotePath string) error {
	if p == nil {
		return fmt.Errorf("GoogleDriveProvider is nil")
	}
	if p.service == nil {
		return fmt.Errorf("Google Drive service is not initialized")
	}

	done := make(chan error)

	go func() {
		file, err := os.Open(localPath)
		if err != nil {
			done <- fmt.Errorf("failed to open file: %w", err)
			return
		}
		defer file.Close()

		folderId, err := p.getOrCreateFolder(remotePath)
		if err != nil {
			done <- fmt.Errorf("failed to create/get folder: %w", err)
			return
		}

		fileMeta := &drive.File{
			Name:    filepath.Base(localPath),
			Parents: []string{folderId},
		}

		_, err = p.service.Files.Create(fileMeta).Media(file).Do()
		if err != nil {
			done <- fmt.Errorf("failed to upload file: %w", err)
			return
		}

		done <- nil
	}()

	// Wait for upload to complete with timeout
	select {
	case err := <-done:
		return err
	case <-time.After(2 * time.Hour): // Long timeout for large files
		return fmt.Errorf("upload timed out after 2 hours")
	}
}

func (p *GoogleDriveProvider) Download(localPath, fileId string) error {
	if !p.isAuthenticated {
		err := p.Authenticate()
		if err != nil {
			return err
		}
	}

	dir := filepath.Dir(localPath)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	file, err := p.service.Files.Get(fileId).Fields("name").Do()
	if err != nil {
		return fmt.Errorf("failed to get file metadata: %w", err)
	}

	localPath = filepath.Join(localPath, file.Name)
	resp, err := p.service.Files.Get(fileId).Download()
	if err != nil {
		return fmt.Errorf("failed to download file: %w", err)
	}
	defer resp.Body.Close()

	outFile, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("failed to create local file: %w", err)
	}
	defer outFile.Close()

	if _, err = io.Copy(outFile, resp.Body); err != nil {
		return fmt.Errorf("failed to save downloaded file: %w", err)
	}

	return nil
}

func (p *GoogleDriveProvider) Delete(fileId string) error {
	if p == nil || p.service == nil {
		return fmt.Errorf("provider not properly initialized")
	}

	if err := p.service.Files.Delete(fileId).Do(); err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}
	return nil
}

func (p *GoogleDriveProvider) ListFiles(folderId string) ([]*drive.File, error) {
	if p == nil || p.service == nil {
		return nil, fmt.Errorf("provider not properly initialized")
	}

	var files []*drive.File
	query := fmt.Sprintf("'%s' in parents", folderId)
	pageToken := ""

	for {
		fileList, err := p.service.Files.List().Q(query).PageToken(pageToken).Do()
		if err != nil {
			return nil, fmt.Errorf("failed to list files: %w", err)
		}

		files = append(files, fileList.Files...)

		if fileList.NextPageToken == "" {
			break
		}
		pageToken = fileList.NextPageToken
	}

	return files, nil
}

func (p *GoogleDriveProvider) getOrCreateFolder(path string) (string, error) {
	// if p == nil || p.service == nil {
	// 	return "", fmt.Errorf("provider not properly initialized")
	// }

	parts := strings.Split(strings.Trim(path, "/"), "/")
	parentId := "root"

	for _, part := range parts {
		if part == "" {
			continue
		}

		escapedName := strings.Replace(part, "'", "\\'", -1)
		query := fmt.Sprintf("name = '%s' and mimeType = 'application/vnd.google-apps.folder' and '%s' in parents and trashed = false",
			escapedName, parentId)

		r, err := p.service.Files.List().Q(query).Fields("files(id, name)").Do()
		if err != nil {
			return "", fmt.Errorf("failed to list folders: %w", err)
		}

		if len(r.Files) > 0 {
			parentId = r.Files[0].Id
			continue
		}

		folderMeta := &drive.File{
			Name:     escapedName,
			MimeType: "application/vnd.google-apps.folder",

			Parents: []string{parentId},
		}

		folder, err := p.service.Files.Create(folderMeta).Do()
		if err != nil {
			return "", fmt.Errorf("failed to create folder: %w", err)
		}
		parentId = folder.Id
	}

	return parentId, nil
}
