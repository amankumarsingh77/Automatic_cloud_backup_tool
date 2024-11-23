package gdrive

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/amankumarsingh77/automated_backup_tool/internal/utils/filesystem"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

type GoogleDriveProvider struct {
	config  *oauth2.Config
	token   *oauth2.Token
	service *drive.Service
}

func NewGoogleDriveProvider() *GoogleDriveProvider {
	return &GoogleDriveProvider{}
}

func (p *GoogleDriveProvider) SetCredentials(clientID, clientSecret, redirectURL string) {
	p.config = &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Scopes: []string{
			drive.DriveScope,
		},
		Endpoint: google.Endpoint,
	}
}

func (p *GoogleDriveProvider) Authenticate() error {
	token, err := filesystem.GetToken("google-drive")
	if err == nil && token.Valid() {
		p.token = token
		client := p.config.Client(context.Background(), p.token)
		if p.service, err = drive.NewService(context.Background(), option.WithHTTPClient(client)); err != nil {
			return fmt.Errorf("failed to create drive service: %v", err)
		}
		return nil
	}

	codeChan := make(chan string, 1)
	errChan := make(chan error, 1)

	server := &http.Server{Addr: ":8080"}

	http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			errChan <- fmt.Errorf("no code in callback")
			w.Write([]byte("Authentication failed. You can close this window."))
			return
		}

		codeChan <- code
		w.Write([]byte("Authentication successful! You can close this window."))

		go func() {
			time.Sleep(time.Second)
			server.Shutdown(context.Background())
		}()
	})

	go func() {
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			errChan <- fmt.Errorf("failed to start server: %v", err)
		}
	}()

	authURL := p.config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	log.Println(authURL)

	if err := openBrowser(authURL); err != nil {
		return fmt.Errorf("failed to open browser: %v", err)
	}

	select {
	case code := <-codeChan:

		token, err := p.config.Exchange(context.Background(), code)
		if err != nil {
			return fmt.Errorf("failed to exchange token: %v", err)
		}
		p.token = token

		err = filesystem.SaveTokenLocally("google-drive", p.token)
		if err != nil {
			return fmt.Errorf("failed to save token locally : %v", err)
		}

		client := p.config.Client(context.Background(), p.token)
		p.service, err = drive.NewService(context.Background(), option.WithHTTPClient(client))
		if err != nil {
			return fmt.Errorf("failed to create drive service: %v", err)
		}

		return nil

	case err := <-errChan:
		return err

	case <-time.After(2 * time.Minute):
		return fmt.Errorf("authentication timed out")
	}
}

func openBrowser(url string) error {
	var err error
	switch runtime.GOOS {
	case "windows":
		err = exec.Command("cmd", "/c", "start", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}
	return err
}

func (p *GoogleDriveProvider) Upload(isSingle bool, localPath, remotePath string) error {
	if p.service == nil {
		err := p.Authenticate()
		if err != nil {
			return err
		}
	}
	done := make(chan error)

	remotePath = strings.ReplaceAll(filepath.Dir(filepath.Clean(remotePath)), "\\", "/")

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

		fileName := filepath.Base(localPath)

		if existingFileId, exists := p.isFileExist(fileName, folderId); exists {
			fileMeta := &drive.File{}
			_, err = p.service.Files.Update(existingFileId, fileMeta).Media(file).Do()
			if err != nil {
				done <- fmt.Errorf("failed to update existing file: %w", err)
				return
			}
		} else {
			fileMeta := &drive.File{
				Name:    fileName,
				Parents: []string{folderId},
			}

			_, err = p.service.Files.Create(fileMeta).Media(file).Do()
			if err != nil {
				done <- fmt.Errorf("failed to upload file: %w", err)
				return
			}
		}

		done <- nil
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(2 * time.Hour):
		return fmt.Errorf("upload timed out after 2 hours")
	}
}

func (p *GoogleDriveProvider) Download(localPath, fileId string) error {
	if p.service == nil {
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
	if p.service == nil {
		err := p.Authenticate()
		if err != nil {
			return err
		}
	}

	if err := p.service.Files.Delete(fileId).Do(); err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}
	return nil
}

func (p *GoogleDriveProvider) ListFiles(folderId string) ([]*drive.File, error) {
	if p.service == nil {
		err := p.Authenticate()
		if err != nil {
			return nil, err
		}
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

func (p *GoogleDriveProvider) isFileExist(path, folderId string) (string, bool) {
	query := fmt.Sprintf("name = '%s' and '%s' in parents and trashed = false",
		strings.Replace(path, "'", "\\'", -1),
		folderId)

	r, err := p.service.Files.List().Q(query).Fields("files(id, name)").Do()
	if err != nil || len(r.Files) == 0 {
		return "", false
	}
	return r.Files[0].Id, true
}
