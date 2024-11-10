package filesystem

import (
	"encoding/json"
	"fmt"
	"golang.org/x/oauth2"
	"log"
	"os"
	"path/filepath"
)

func SaveTokenLocally(provider string, token *oauth2.Token) error {
	dir, _ := os.Getwd()
	tokenFile := filepath.Join(dir, os.Getenv("APP_NAME"), provider+"-token.json")
	log.Println("Saving token to ", tokenFile)
	dir = filepath.Dir(tokenFile)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return err
	}
	file, err := os.Create(tokenFile)
	if err != nil {
		return fmt.Errorf("Error occured while saving token: %v\n", err.Error())
	}
	log.Println(file.Name())
	defer file.Close()
	if err := json.NewEncoder(file).Encode(token); err != nil {
		return fmt.Errorf("Error occured while saving token: %v\n", err.Error())
	}
	fmt.Println("Token saved locally!")
	return nil
}

func GetToken(provider string) (*oauth2.Token, error) {
	dir, _ := os.Getwd()
	tokenFile := filepath.Join(dir, os.Getenv("APP_NAME"), provider+"-token.json")
	jsonFile, err := os.Open(tokenFile)
	if err != nil {
		return nil, fmt.Errorf("Error occured while opening token file: %v\n", err.Error())
	}
	defer jsonFile.Close()
	token := &oauth2.Token{}
	err = json.NewDecoder(jsonFile).Decode(token)
	if err != nil {
		return nil, fmt.Errorf("Error occured while decoding token file: %v\n", err.Error())
	}
	return token, nil
}
