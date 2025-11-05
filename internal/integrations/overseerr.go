package integrations

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

type OverseerrClient struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

type OverseerrRequest struct {
	ID          int    `json:"id"`
	MediaID     int    `json:"mediaId"`
	MediaType   string `json:"mediaType"`
	Status      int    `json:"status"`
	RequestedBy struct {
		ID       int    `json:"id"`
		Email    string `json:"email"`
		Username string `json:"username"`
		PlexID   *int   `json:"plexId"`
	} `json:"requestedBy"`
	Media struct {
		ID       int    `json:"id"`
		TMDBID   int    `json:"tmdbId"`
		TVDBID   *int   `json:"tvdbId"`
		Title    string `json:"title"`
		MediaType string `json:"mediaType"`
	} `json:"media"`
}

type OverseerrMedia struct {
	ID       int    `json:"id"`
	TMDBID   int    `json:"tmdbId"`
	TVDBID   *int   `json:"tvdbId"`
	Title    string `json:"title"`
	MediaType string `json:"mediaType"`
}

func NewOverseerrClient(baseURL, apiKey string) *OverseerrClient {
	return &OverseerrClient{
		baseURL: baseURL,
		apiKey:  apiKey,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *OverseerrClient) makeRequest(method, endpoint string, body interface{}) (*http.Response, error) {
	url := fmt.Sprintf("%s/api/v1%s", c.baseURL, endpoint)
	
	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", c.apiKey)

	return c.client.Do(req)
}

// GetRequests fetches all requests from Overseerr
func (c *OverseerrClient) GetRequests() ([]OverseerrRequest, error) {
	resp, err := c.makeRequest("GET", "/request", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("overseerr API error: %s - %s", resp.Status, string(body))
	}

	var result struct {
		Results []OverseerrRequest `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Results, nil
}

// GetRequestByID fetches a specific request
func (c *OverseerrClient) GetRequestByID(id int) (*OverseerrRequest, error) {
	resp, err := c.makeRequest("GET", fmt.Sprintf("/request/%d", id), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("overseerr API error: %s - %s", resp.Status, string(body))
	}

	var request OverseerrRequest
	if err := json.NewDecoder(resp.Body).Decode(&request); err != nil {
		return nil, err
	}

	return &request, nil
}

// DeleteRequest deletes a request from Overseerr
func (c *OverseerrClient) DeleteRequest(id int) error {
	resp, err := c.makeRequest("DELETE", fmt.Sprintf("/request/%d", id), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("overseerr API error: %s - %s", resp.Status, string(body))
	}

	return nil
}

// FindRequestByMediaID finds an Overseerr request by TMDB ID (for movies) or TVDB ID (for series)
func (c *OverseerrClient) FindRequestByMediaID(tmdbID *int, tvdbID *int, mediaType string) (*OverseerrRequest, error) {
	// Get all requests
	requests, err := c.GetRequests()
	if err != nil {
		return nil, fmt.Errorf("failed to get requests: %w", err)
	}

	// Log all requests for debugging
	slog.Info("Searching Overseerr requests", "count", len(requests), "tmdb_id", tmdbID, "tvdb_id", tvdbID, "media_type", mediaType)
	for i, req := range requests {
		slog.Info("Overseerr request", 
			"index", i, 
			"id", req.ID, 
			"media_type", req.MediaType, 
			"media_tmdb_id", req.Media.TMDBID, 
			"media_tvdb_id", req.Media.TVDBID, 
			"media_title", req.Media.Title,
			"status", req.Status)
	}

	// Search for a matching request
	for _, req := range requests {
		// Check if media type matches
		// Overseerr uses "movie" for movies and "tv" for series
		// Also check Media.MediaType as it might be populated when req.MediaType is empty
		requestMediaType := req.MediaType
		if requestMediaType == "" {
			requestMediaType = req.Media.MediaType
		}

		expectedType := mediaType
		if mediaType == "series" {
			expectedType = "tv" // Overseerr uses "tv" for series
		}

		// Check if media type matches (allow empty string or match)
		typeMatches := false
		if requestMediaType == "" {
			// If type is empty, we'll match by ID only (fallback)
			typeMatches = true
			slog.Debug("Request has empty media type, matching by ID only", "request_id", req.ID)
		} else if requestMediaType == expectedType || requestMediaType == mediaType {
			typeMatches = true
		}

		if !typeMatches {
			slog.Debug("Skipping request - media type mismatch", 
				"request_id", req.ID,
				"request_type", requestMediaType, 
				"expected", expectedType, 
				"our_type", mediaType)
			continue
		}

		// For movies, match by TMDB ID
		if mediaType == "movie" && tmdbID != nil && req.Media.TMDBID == *tmdbID {
			slog.Info("Found matching Overseerr request", "request_id", req.ID, "tmdb_id", *tmdbID)
			return &req, nil
		}

		// For series, match by TVDB ID
		if mediaType == "series" && tvdbID != nil && req.Media.TVDBID != nil && *req.Media.TVDBID == *tvdbID {
			slog.Info("Found matching Overseerr request", "request_id", req.ID, "tvdb_id", *tvdbID)
			return &req, nil
		}
	}

	return nil, nil // No matching request found
}

