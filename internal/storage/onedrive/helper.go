package onedrive

import (
	"io"
	"net/http"

	"golang.org/x/oauth2"
)

const ONEDRIVE_BASE_URL = "https://graph.microsoft.com/v1.0/me/drive/"

func makeRequest(method, url string, token *oauth2.Token, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, ONEDRIVE_BASE_URL+url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", "Bearer "+token.AccessToken)
	if method == http.MethodPut {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	client := &http.Client{}
	return client.Do(req)
}
