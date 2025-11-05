package integrations

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

type QBittorrentClient struct {
	baseURL  string
	username string
	password string
	client   *http.Client
	sid      string // session ID
}

type QBittorrentTorrent struct {
	Hash           string  `json:"hash"`
	Name           string  `json:"name"`
	Size           int64   `json:"size"`
	State          string  `json:"state"` // e.g., "uploading", "downloading", "pausedUP"
	SeedingTime    int64   `json:"seeding_time"` // in seconds
	Uploaded       int64   `json:"uploaded"`
	Downloaded     int64   `json:"downloaded"`
	Ratio          float64 `json:"ratio"`
	AddedOn        int64   `json:"added_on"` // Unix timestamp
	Tracker        string  `json:"tracker"`
	Category       string  `json:"category"`
	Tags           string  `json:"tags"`
	ContentPath    string  `json:"content_path"`
}

type QBittorrentTorrentInfo struct {
	Hash           string  `json:"hash"`
	Name           string  `json:"name"`
	Size           int64   `json:"size"`
	State          string  `json:"state"`
	SeedingTime    int64   `json:"seeding_time"`
	Uploaded       int64   `json:"uploaded"`
	Downloaded     int64   `json:"downloaded"`
	Ratio          float64 `json:"ratio"`
	AddedOn        int64   `json:"added_on"`
	Tracker        string  `json:"tracker"`
	Category       string  `json:"category"`
	Tags           string  `json:"tags"`
	ContentPath    string  `json:"content_path"`
}

func NewQBittorrentClient(baseURL, username, password string) *QBittorrentClient {
	return &QBittorrentClient{
		baseURL:  baseURL,
		username: username,
		password: password,
		client:  newHTTPClient(30 * time.Second),
	}
}

func (c *QBittorrentClient) login() error {
	data := url.Values{}
	data.Set("username", c.username)
	data.Set("password", c.password)

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/api/v2/auth/login", c.baseURL), bytes.NewBufferString(data.Encode()))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// qBittorrent sets cookies for session
	cookies := resp.Cookies()
	for _, cookie := range cookies {
		if cookie.Name == "SID" {
			c.sid = cookie.Value
		}
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("qBittorrent login failed: %s", resp.Status)
	}

	return nil
}

func (c *QBittorrentClient) ensureLoggedIn() error {
	if c.sid == "" {
		return c.login()
	}
	return nil
}

func (c *QBittorrentClient) makeRequest(method, endpoint string) (*http.Response, error) {
	if err := c.ensureLoggedIn(); err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/api/v2%s", c.baseURL, endpoint)
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}

	// Set cookie for session
	req.AddCookie(&http.Cookie{
		Name:  "SID",
		Value: c.sid,
	})

	return c.client.Do(req)
}

// GetTorrents fetches all torrents from qBittorrent
func (c *QBittorrentClient) GetTorrents() ([]QBittorrentTorrent, error) {
	resp, err := c.makeRequest("GET", "/torrents/info")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("qbittorrent API error: %s - %s", resp.Status, string(body))
	}

	var torrents []QBittorrentTorrent
	if err := json.NewDecoder(resp.Body).Decode(&torrents); err != nil {
		return nil, err
	}

	return torrents, nil
}

// GetTorrentProperties fetches detailed properties of a torrent
func (c *QBittorrentClient) GetTorrentProperties(hash string) (*QBittorrentTorrentInfo, error) {
	resp, err := c.makeRequest("GET", fmt.Sprintf("/torrents/properties?hash=%s", hash))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("qbittorrent API error: %s - %s", resp.Status, string(body))
	}

	var props QBittorrentTorrentInfo
	if err := json.NewDecoder(resp.Body).Decode(&props); err != nil {
		return nil, err
	}

	return &props, nil
}

// DeleteTorrent deletes a torrent and optionally its files
func (c *QBittorrentClient) DeleteTorrent(hash string, deleteFiles bool) error {
	endpoint := fmt.Sprintf("/torrents/delete?hashes=%s&deleteFiles=%t", hash, deleteFiles)
	resp, err := c.makeRequest("GET", endpoint)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("qbittorrent API error: %s - %s", resp.Status, string(body))
	}

	return nil
}

