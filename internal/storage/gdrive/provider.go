package gdrive

import (
	"context"
	"fmt"
	"github.com/joho/godotenv"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
	"io"
	"log"
	"os"
	"path/filepath"
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
	config := oauth2.Config{
		ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		Endpoint:     google.Endpoint,
		RedirectURL:  os.Getenv("GOOGLE_CLIENT_REDIRECT_URL"),
		Scopes:       []string{drive.DriveScope},
	}

	authUrl := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Visit the following URL to authenticate: %s\n", authUrl)
	var code string
	fmt.Println("Enter the code below:")
	fmt.Scanln(&code)
	token, err := config.Exchange(context.Background(), code)
	if err != nil {
		return fmt.Errorf("error occured while generating token : %v\n", err.Error())
	}
	p.token = token
	client := config.Client(context.Background(), p.token)
	p.service, err = drive.NewService(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		return fmt.Errorf("error occured while creating drive service : %v \n", err.Error())
	}

	if err = SaveTokenLocally(p.token); err != nil {
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

	fileMeta := &drive.File{
		Name:    file.Name(),
		Parents: []string{remotePath},
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
	fileList, err := p.service.Files.List().Q(query).Do()
	if err != nil {
		return nil, fmt.Errorf("error occured while listing files : %v \n", err.Error())
	}
	files = append(files, fileList.Files...)
	return files, nil
}
