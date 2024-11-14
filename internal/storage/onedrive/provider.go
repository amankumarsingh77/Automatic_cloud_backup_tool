package onedrive

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/amankumarsingh77/automated_backup_tool/internal/utils/filesystem"
	"github.com/joho/godotenv"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/microsoft"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type OneDriveProvider struct {
	client          *http.Client
	config          *oauth2.Config
	token           *oauth2.Token
	isAuthenticated bool
}

type DriveItem struct {
	DownloadUrl     string    `json:"@microsoft.graph.downloadUrl"`
	CreatedDateTime time.Time `json:"createdDateTime"`
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	ParentReference ParentRef `json:"parentReference"`
	WebUrl          string    `json:"webUrl"`
	File            FileInfo  `json:"file"`
	Size            int64     `json:"size"`
}

type User struct {
	Email       string `json:"email"`
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
}

type ParentRef struct {
	DriveType string `json:"driveType"`
	ID        string `json:"id"`
	Name      string `json:"name"`
	Path      string `json:"path"`
}

type FileInfo struct {
	MimeType string `json:"mimeType"`
}

func init() {
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatal("Error loading .env file")
	}
}

func NewOneDriveProvider() *OneDriveProvider {
	return &OneDriveProvider{}
}

func (p *OneDriveProvider) Authenticate() error {
	// For some reason there is no need to provide client_secret ;)
	config := &oauth2.Config{
		ClientID: os.Getenv("ONEDRIVE_CLIENT_ID"),
		//ClientSecret: os.Getenv("ONEDRIVE_CLIENT_SECRET"),
		Endpoint:    microsoft.AzureADEndpoint("common"),
		RedirectURL: os.Getenv("ONEDRIVE_REDIRECT_URL"),
		Scopes:      []string{"offline_access", "Files.ReadWrite"},
	}

	token, err := filesystem.GetToken("one-drive")
	if err == nil {
		p.token = token
		ts := oauth2.StaticTokenSource(p.token)
		p.client = oauth2.NewClient(context.Background(), ts)
		return nil
	}

	authUrl := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline, oauth2.ApprovalForce)
	fmt.Printf("Visit the URL for the auth dialog: %v", authUrl)
	var code string
	fmt.Println("Enter the code below:")
	fmt.Scanln(&code)
	token, err = config.Exchange(context.Background(), code)
	if err != nil {
		return fmt.Errorf("error occured while generating token : %v\n", err.Error())
	}
	err = filesystem.SaveTokenLocally("one-drive", token)
	if err != nil {
		return fmt.Errorf("error saving token: %v\n", err)
	}
	p.token = token
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: p.token.AccessToken})
	p.client = oauth2.NewClient(context.Background(), ts)
	p.isAuthenticated = true
	fmt.Println("Authentication successful and token saved")
	return nil
}

func (p *OneDriveProvider) Upload(localPath, remotePath string) error {
	if !p.isAuthenticated {
		err := p.Authenticate()
		if err != nil {
			return err
		}
	}
	file, err := os.Open(localPath)
	filename := filepath.Base(localPath)
	if err != nil {
		return fmt.Errorf("could not open file: %v", err.Error())
	}
	defer file.Close()
	url := fmt.Sprintf("items/root:/%s/%s:/content", remotePath, filename)
	resp, err := makeRequest("PUT", url, p.token, file)
	if err != nil {
		return fmt.Errorf("could not upload file: %v", err.Error())
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("could not upload file: %v", body)
	}
	fmt.Println("File Uploaded successfully!")
	return nil
}

func (p *OneDriveProvider) Download(localPath, fileId string) error {
	dir, _ := os.Getwd()
	if err := os.Mkdir(dir, os.ModePerm); err != nil && !os.IsExist(err) {
		return fmt.Errorf("could not create directory: %v", err.Error())
	}
	url := fmt.Sprintf("items/%s/content", fileId)
	resp, err := makeRequest(http.MethodGet, url, p.token, nil)
	if err != nil {
		return fmt.Errorf("could not download file: %v", err.Error())
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("could not download file: %v", resp.Status)
	}
	_, params, err := mime.ParseMediaType(resp.Header.Get("Content-Disposition"))

	var filename string
	if err == nil {
		filename = filepath.Base(params["filename"])
	} else {
		filename = fileId
	}

	localPath = filepath.Join(dir, filename)
	file, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("could not create file: %v", err.Error())
	}
	defer file.Close()
	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return fmt.Errorf("could not download file: %v", err.Error())
	}
	fmt.Println("File Downloaded successfully!")
	return nil
}

func (p *OneDriveProvider) ListFiles(folderId string) ([]*DriveItem, error) {
	var result struct {
		Value []*DriveItem `json:"value"`
	}

	query := fmt.Sprintf("items/%s/children", folderId)
	resp, err := makeRequest(http.MethodGet, query, p.token, nil)
	if err != nil {
		return nil, fmt.Errorf("could not list files: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("could not decode files: %w", err)
	}

	return result.Value, nil
}
