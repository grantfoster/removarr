package integrations

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

type TautulliClient struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

type TautulliHistory struct {
	MediaType    string `json:"media_type"`
	Title        string `json:"title"`
	User         string `json:"user"`
	LastPlayed   int64  `json:"last_played"` // Unix timestamp
	PlayCount    int    `json:"play_count"`
	TMDBID       *int   `json:"tmdb_id"`
	TVDBID       *int   `json:"tvdb_id"`
}

type TautulliHistoryResponse struct {
	Response struct {
		Data []TautulliHistory `json:"data"`
	} `json:"response"`
}

func NewTautulliClient(baseURL, apiKey string) *TautulliClient {
	return &TautulliClient{
		baseURL: baseURL,
		apiKey:  apiKey,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *TautulliClient) makeRequest(method string, params map[string]string) (*http.Response, error) {
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, err
	}

	u.Path = "/api/v2"
	q := u.Query()
	q.Set("apikey", c.apiKey)
	for k, v := range params {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequest(method, u.String(), nil)
	if err != nil {
		return nil, err
	}

	return c.client.Do(req)
}

// GetHistory fetches watch history from Tautulli
func (c *TautulliClient) GetHistory() ([]TautulliHistory, error) {
	resp, err := c.makeRequest("GET", map[string]string{
		"cmd": "get_history",
		"length": "10000", // Get a lot of history
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("tautulli API error: %s - %s", resp.Status, string(body))
	}

	var result TautulliHistoryResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Response.Data, nil
}

// GetHistoryByUser fetches watch history for a specific user
func (c *TautulliClient) GetHistoryByUser(username string) ([]TautulliHistory, error) {
	resp, err := c.makeRequest("GET", map[string]string{
		"cmd": "get_history",
		"user": username,
		"length": "10000",
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("tautulli API error: %s - %s", resp.Status, string(body))
	}

	var result TautulliHistoryResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Response.Data, nil
}

