package integrations

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type SonarrClient struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

type SonarrSeries struct {
	ID              int    `json:"id"`
	Title           string `json:"title"`
	Path            string `json:"path"`
	TVDBID          int    `json:"tvdbId"`
	Monitored       bool   `json:"monitored"`
	Status          string `json:"status"`
	Added           string `json:"added"`
	Statistics      *SonarrStatistics `json:"statistics"`
}

type SonarrStatistics struct {
	SizeOnDisk int64 `json:"sizeOnDisk"`
}

func NewSonarrClient(baseURL, apiKey string) *SonarrClient {
	return &SonarrClient{
		baseURL: baseURL,
		apiKey:  apiKey,
		client:  newHTTPClient(30 * time.Second),
	}
}

func (c *SonarrClient) makeRequest(method, endpoint string) (*http.Response, error) {
	url := fmt.Sprintf("%s/api/v3%s?apikey=%s", c.baseURL, endpoint, c.apiKey)
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	return c.client.Do(req)
}

// GetSeries fetches all series from Sonarr
func (c *SonarrClient) GetSeries() ([]SonarrSeries, error) {
	resp, err := c.makeRequest("GET", "/series")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("sonarr API error: %s - %s", resp.Status, string(body))
	}

	var series []SonarrSeries
	if err := json.NewDecoder(resp.Body).Decode(&series); err != nil {
		return nil, err
	}

	return series, nil
}

// GetSeriesByID fetches a specific series
func (c *SonarrClient) GetSeriesByID(id int) (*SonarrSeries, error) {
	resp, err := c.makeRequest("GET", fmt.Sprintf("/series/%d", id))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("sonarr API error: %s - %s", resp.Status, string(body))
	}

	var series SonarrSeries
	if err := json.NewDecoder(resp.Body).Decode(&series); err != nil {
		return nil, err
	}

	return &series, nil
}

// DeleteSeries deletes a series and its files
// addImportExclusion=false prevents the series from being added to the exclusion list
func (c *SonarrClient) DeleteSeries(id int, deleteFiles bool, addImportExclusion bool) error {
	endpoint := fmt.Sprintf("/series/%d?deleteFiles=%t&addImportExclusion=%t", id, deleteFiles, addImportExclusion)
	resp, err := c.makeRequest("DELETE", endpoint)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("sonarr API error: %s - %s", resp.Status, string(body))
	}

	return nil
}

// UnmonitorSeries unmonitors a series
func (c *SonarrClient) UnmonitorSeries(id int) error {
	series, err := c.GetSeriesByID(id)
	if err != nil {
		return err
	}

	series.Monitored = false
	url := fmt.Sprintf("%s/api/v3/series?apikey=%s", c.baseURL, c.apiKey)
	jsonData, err := json.Marshal(series)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("PUT", url, io.NopCloser(bytes.NewReader(jsonData)))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("sonarr API error: %s - %s", resp.Status, string(body))
	}

	return nil
}

