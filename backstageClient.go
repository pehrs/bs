package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// ── Backstage client ──────────────────────────────────────────────────────────

type backstageClient struct {
	baseURL string
	token   string
	http    *http.Client
}

func newBackstageClient(baseURL, token string) backstageClient {
	return backstageClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		http:    &http.Client{},
	}
}

// fetchPage fetches one page. cursor="" means first page. Returns nextCursor="" when no more pages.
func (c backstageClient) fetchPage(kind, cursor string) ([]Entity, int, string, error) {
	endpoint := c.baseURL + "/api/catalog/entities/by-query"

	params := url.Values{}
	params.Set("limit", "200")
	if kind != "" && kind != "All" {
		params.Set("filter", "kind="+kind)
	}
	if cursor != "" {
		params.Set("cursor", cursor)
	}

	req, err := http.NewRequest("GET", endpoint+"?"+params.Encode(), nil)
	if err != nil {
		return nil, 0, "", fmt.Errorf("creating request: %w", err)
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, "", fmt.Errorf("connecting to Backstage: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, 0, "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var result queryResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, 0, "", fmt.Errorf("decoding response: %w", err)
	}
	return result.Items, result.TotalItems, result.PageInfo.NextCursor, nil
}
