package integrations

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type PlexClient struct {
	baseURL string
	token   string
	client  *http.Client
}

type PlexUser struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Thumb    string `json:"thumb"`
}

type PlexUsersResponse struct {
	MediaContainer struct {
		Users []PlexUser `json:"User"`
	} `json:"MediaContainer"`
}

func NewPlexClient(baseURL, token string) *PlexClient {
	return &PlexClient{
		baseURL: baseURL,
		token:   token,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *PlexClient) makeRequest(method, endpoint string) (*http.Response, error) {
	url := fmt.Sprintf("%s%s", c.baseURL, endpoint)
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-Plex-Token", c.token)
	req.Header.Set("Accept", "application/json")

	return c.client.Do(req)
}

// GetUsers fetches all users from Plex
func (c *PlexClient) GetUsers() ([]PlexUser, error) {
	resp, err := c.makeRequest("GET", "/api/users")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("plex API error: %s - %s", resp.Status, string(body))
	}

	var result PlexUsersResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.MediaContainer.Users, nil
}

// VerifyToken verifies if the Plex token is valid
func (c *PlexClient) VerifyToken() (bool, error) {
	resp, err := c.makeRequest("GET", "/api/v2/user")
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK, nil
}

