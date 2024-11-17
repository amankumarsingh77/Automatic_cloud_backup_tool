package config

import (
	"os"

	"github.com/joho/godotenv"
)

type GoogleDriveConfig struct {
	ClientID                   string `yaml:"client_id"`
	ClientSecret               string `yaml:"client_secret"`
	GOOGLE_CLIENT_REDIRECT_URL string `yaml:"google_client_redirect_url"`
}

type OneDriveConfig struct {
	ClientID                     string `yaml:"client_id"`
	ONEDRIVE_CLIENT_REDIRECT_URL string `yaml:"onedrive_client_redirect_url"`
}

type Config struct {
	Providers struct {
		GoogleDrive GoogleDriveConfig
		OneDrive    OneDriveConfig
	}
}

func LoadConfig() *Config {
	godotenv.Load()
	return &Config{
		Providers: struct {
			GoogleDrive GoogleDriveConfig
			OneDrive    OneDriveConfig
		}{
			GoogleDrive: GoogleDriveConfig{
				ClientID:                   os.Getenv("GOOGLE_CLIENT_ID"),
				ClientSecret:               os.Getenv("GOOGLE_CLIENT_SECRET"),
				GOOGLE_CLIENT_REDIRECT_URL: os.Getenv("GOOGLE_CLIENT_REDIRECT_URL"),
			},
			OneDrive: OneDriveConfig{
				ClientID:                     os.Getenv("ONEDRIVE_CLIENT_ID"),
				ONEDRIVE_CLIENT_REDIRECT_URL: os.Getenv("ONEDRIVE_CLIENT_REDIRECT_URL"),
			},
		},
	}
}
