package integrations

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type ProwlarrClient struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

type ProwlarrIndexer struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Protocol    string `json:"protocol"` // "torrent" or "usenet"
	Privacy     string `json:"privacy"`  // "private" or "public"
	MinSeedTime *int64 `json:"minSeedTime"` // in seconds
	MinRatio    *float64 `json:"minRatio"`
}

func NewProwlarrClient(baseURL, apiKey string) *ProwlarrClient {
	return &ProwlarrClient{
		baseURL: baseURL,
		apiKey:  apiKey,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *ProwlarrClient) makeRequest(method, endpoint string) (*http.Response, error) {
	url := fmt.Sprintf("%s/api/v1%s?apikey=%s", c.baseURL, endpoint, c.apiKey)
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	return c.client.Do(req)
}

// GetIndexers fetches all indexers from Prowlarr
func (c *ProwlarrClient) GetIndexers() ([]ProwlarrIndexer, error) {
	resp, err := c.makeRequest("GET", "/indexer")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("prowlarr API error: %s - %s", resp.Status, string(body))
	}

	var indexers []ProwlarrIndexer
	if err := json.NewDecoder(resp.Body).Decode(&indexers); err != nil {
		return nil, err
	}

	return indexers, nil
}

// GetIndexerByID fetches a specific indexer
func (c *ProwlarrClient) GetIndexerByID(id int) (*ProwlarrIndexer, error) {
	resp, err := c.makeRequest("GET", fmt.Sprintf("/indexer/%d", id))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("prowlarr API error: %s - %s", resp.Status, string(body))
	}

	var indexer ProwlarrIndexer
	if err := json.NewDecoder(resp.Body).Decode(&indexer); err != nil {
		return nil, err
	}

	return &indexer, nil
}

