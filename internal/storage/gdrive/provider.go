package gdrive

import (
	"context"
	"fmt"
	"github.com/amankumarsingh77/automated_backup_tool/internal/utils/filesystem"
	"github.com/joho/godotenv"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
)

type GoogleDriveProvider struct {
	service *drive.Service
	token   *oauth2.Token
}

func NewGoogleDriveProvider() *GoogleDriveProvider {
	return &GoogleDriveProvider{}
}

func init() {
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatal("Error loading .env file")
	}
}

func (p *GoogleDriveProvider) Authenticate() error {
	token, err := filesystem.GetToken("google-drive")
	config := oauth2.Config{
		ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		Endpoint:     google.Endpoint,
		RedirectURL:  os.Getenv("GOOGLE_CLIENT_REDIRECT_URL"),
		Scopes:       []string{drive.DriveScope},
	}
	if err == nil {
		p.token = token
		client := config.Client(context.Background(), p.token)
		p.service, err = drive.NewService(context.Background(), option.WithHTTPClient(client))
		if err != nil {
			return fmt.Errorf("error occured while creating drive service : %v \n", err.Error())
		}
		return nil
	}

	authUrl := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Visit the following URL to authenticate: %s\n", authUrl)
	var code string
	fmt.Println("Enter the code below:")
	fmt.Scanln(&code)
	token, err = config.Exchange(context.Background(), code)
	if err != nil {
		return fmt.Errorf("error occured while generating token : %v\n", err.Error())
	}
	p.token = token
	client := config.Client(context.Background(), p.token)
	p.service, err = drive.NewService(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		return fmt.Errorf("error occured while creating drive service : %v \n", err.Error())
	}

	if err = filesystem.SaveTokenLocally("google-drive", p.token); err != nil {
		return fmt.Errorf("error occured while saving token locally : %v \n", err.Error())
	}

	fmt.Println("Authentication successful and token saved")
	return nil
}

func (p *GoogleDriveProvider) Upload(localPath, remotePath string) error {
	file, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("error occured while opening file : %v \n", err.Error())
	}
	defer file.Close()

	folderId, err := p.getOrCreateFolder(remotePath)
	if err != nil {
		return fmt.Errorf("error occured while creating folder : %v \n", err.Error())
	}
	fileMeta := &drive.File{
		Name:    filepath.Base(localPath),
		Parents: []string{folderId},
	}
	_, err = p.service.Files.Create(fileMeta).Media(file).Do()
	// TODO: Implement upload logs to a json file
	if err != nil {
		return fmt.Errorf("error occured while uploading file : %v \n", err.Error())
	}
	fmt.Println("File uploaded successfully")
	return nil
}

func (p *GoogleDriveProvider) Download(localPath, fileId string) error {
	dir := filepath.Dir(localPath)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil && os.IsExist(err) {
		return fmt.Errorf("error occured while creating directory : %v \n", err.Error())
	}

	file, err := p.service.Files.Get(fileId).Fields("name").Do()
	if err != nil {
		return fmt.Errorf("error occured while downloading file : %v \n", err.Error())
	}

	localPath = filepath.Join(localPath, file.Name)
	resp, err := p.service.Files.Get(fileId).Download()
	if err != nil {
		return fmt.Errorf("error occured while downloading file : %v \n", err.Error())
	}
	defer resp.Body.Close()

	outFile, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("error occured while creating file : %v \n", err.Error())
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, resp.Body)
	if err != nil {
		return fmt.Errorf("error occured while downloading file : %v \n", err.Error())
	}
	return nil
}

func (p *GoogleDriveProvider) Delete(fileId string) error {
	err := p.service.Files.Delete(fileId).Do()
	if err != nil {
		return fmt.Errorf("error occured while deleting file : %v \n", err.Error())
	}
	return nil
}

func (p *GoogleDriveProvider) ListFiles(folderId string) ([]*drive.File, error) {
	var files []*drive.File

	query := fmt.Sprintf("'%s' in parents", folderId)
	pageToken := ""

	for {
		fileList, err := p.service.Files.List().Q(query).PageToken(pageToken).Do()
		if err != nil {
			return nil, fmt.Errorf("error occurred while listing files: %v", err)
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
	var parentId string = "root"

	log.Println(parts)

	for _, part := range parts {
		if part == "" {
			continue
		}
		escapedName := strings.Replace(part, "'", "\\'", -1)
		query := fmt.Sprintf("name = '%s' and mimeType = 'application/vnd.google-apps.folder' and '%s' in parents and trashed = false", escapedName, parentId)
		log.Println("query :", query)
		r, err := p.service.Files.List().Q(query).Fields("files(id, name)").Do()
		if err != nil {
			return "", fmt.Errorf("error occured while listing files: %v", err)
		}
		if len(r.Files) > 0 {
			parentId = r.Files[0].Id
			continue
		}
		folderMeta := &drive.File{
			Name:     escapedName,
			MimeType: "application/vnd.google-apps.folder",
			Parents:  []string{parentId},
		}
		folder, err := p.service.Files.Create(folderMeta).Do()
		if err != nil {
			return "", fmt.Errorf("error occured while creating folder : %v \n", err.Error())
		}
		parentId = folder.Id
	}
	return parentId, nil
}
