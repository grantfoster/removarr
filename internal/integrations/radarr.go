package integrations

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type RadarrClient struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

type RadarrMovie struct {
	ID               int               `json:"id"`
	Title            string            `json:"title"`
	Path             string            `json:"path"`
	TMDBID           int               `json:"tmdbId"`
	Monitored        bool              `json:"monitored"`
	Status           string            `json:"status"`
	Added            string            `json:"added"`
	QualityProfileID int               `json:"qualityProfileId"`
	RootFolderPath  string            `json:"rootFolderPath"`
	Statistics       *RadarrStatistics `json:"statistics"`
}

type RadarrStatistics struct {
	SizeOnDisk int64 `json:"sizeOnDisk"`
}

func NewRadarrClient(baseURL, apiKey string) *RadarrClient {
	return &RadarrClient{
		baseURL: baseURL,
		apiKey:  apiKey,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *RadarrClient) makeRequest(method, endpoint string) (*http.Response, error) {
	url := fmt.Sprintf("%s/api/v3%s?apikey=%s", c.baseURL, endpoint, c.apiKey)
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	return c.client.Do(req)
}

// GetMovies fetches all movies from Radarr
func (c *RadarrClient) GetMovies() ([]RadarrMovie, error) {
	resp, err := c.makeRequest("GET", "/movie")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("radarr API error: %s - %s", resp.Status, string(body))
	}

	var movies []RadarrMovie
	if err := json.NewDecoder(resp.Body).Decode(&movies); err != nil {
		return nil, err
	}

	return movies, nil
}

// GetMovieByID fetches a specific movie
func (c *RadarrClient) GetMovieByID(id int) (*RadarrMovie, error) {
	resp, err := c.makeRequest("GET", fmt.Sprintf("/movie/%d", id))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("radarr API error: %s - %s", resp.Status, string(body))
	}

	var movie RadarrMovie
	if err := json.NewDecoder(resp.Body).Decode(&movie); err != nil {
		return nil, err
	}

	return &movie, nil
}

// DeleteMovie deletes a movie and its files
// addImportExclusion=false prevents the movie from being added to the exclusion list
func (c *RadarrClient) DeleteMovie(id int, deleteFiles bool, addImportExclusion bool) error {
	// Use makeRequest which handles API key authentication
	endpoint := fmt.Sprintf("/movie/%d?deleteFiles=%t&addImportExclusion=%t", id, deleteFiles, addImportExclusion)
	resp, err := c.makeRequest("DELETE", endpoint)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Radarr returns 200 OK on successful deletion
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("radarr API error: %s - %s", resp.Status, string(body))
	}

	return nil
}

// UnmonitorMovie unmonitors a movie
func (c *RadarrClient) UnmonitorMovie(id int) error {
	// Get the full movie object to preserve all required fields
	movie, err := c.GetMovieByID(id)
	if err != nil {
		return fmt.Errorf("failed to get movie: %w", err)
	}

	// Update monitored status
	movie.Monitored = false
	
	// Ensure QualityProfileID and RootFolderPath are set (required by Radarr API)
	// If they're missing from GetMovieByID response, fetch defaults
	if movie.QualityProfileID == 0 {
		// Try to get quality profile from all movies
		movies, err := c.GetMovies()
		if err == nil && len(movies) > 0 {
			// Use the quality profile from the first movie as a fallback
			movie.QualityProfileID = movies[0].QualityProfileID
			if movie.QualityProfileID == 0 {
				movie.QualityProfileID = 1 // Default to profile 1
			}
		} else {
			movie.QualityProfileID = 1 // Default fallback
		}
	}
	
	if movie.RootFolderPath == "" {
		// Try to get root folder from all movies
		movies, err := c.GetMovies()
		if err == nil && len(movies) > 0 {
			movie.RootFolderPath = movies[0].RootFolderPath
			if movie.RootFolderPath == "" {
				movie.RootFolderPath = "/movies" // Default fallback
			}
		} else {
			movie.RootFolderPath = "/movies" // Default fallback
		}
	}
	
	// Use the standard PUT endpoint with full movie object
	url := fmt.Sprintf("%s/api/v3/movie?apikey=%s", c.baseURL, c.apiKey)
	jsonData, err := json.Marshal(movie)
	if err != nil {
		return fmt.Errorf("failed to marshal movie: %w", err)
	}

	req, err := http.NewRequest("PUT", url, io.NopCloser(bytes.NewReader(jsonData)))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Radarr returns 200 OK or 202 Accepted for successful PUT requests
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("radarr API error: %s - %s", resp.Status, string(body))
	}

	return nil
}
