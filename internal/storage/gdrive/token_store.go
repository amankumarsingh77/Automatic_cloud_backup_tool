package gdrive

import (
	"encoding/json"
	"fmt"
	"golang.org/x/oauth2"
	"os"
	"path/filepath"
)

var tokenFile = filepath.Join(os.Getenv("HOME"), os.Getenv("APP_NAME"), "gdrive_creds.json")

func SaveTokenLocally(token *oauth2.Token) error {
	dir := filepath.Dir(tokenFile)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return err
	}
	file, err := os.Create(tokenFile)
	if err != nil {
		return fmt.Errorf("Error occured while saving token: %v\n", err.Error())
	}
	defer file.Close()
	if err := json.NewEncoder(file).Encode(token); err != nil {
		return fmt.Errorf("Error occured while saving token: %v\n", err.Error())
	}
	fmt.Println("Token saved locally!")
	return nil
}
