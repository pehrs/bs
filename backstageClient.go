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

// queryPage executes a GET /api/catalog/entities/by-query request with the
// given params and returns the items, total count, and next cursor.
func (c backstageClient) queryPage(params url.Values) ([]Entity, int, string, error) {
	endpoint := c.baseURL + "/api/catalog/entities/by-query"

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

// fetchPage fetches one catalog page filtered by kind.
// cursor="" means first page; returns nextCursor="" when no more pages exist.
func (c backstageClient) fetchPage(kind, cursor string) ([]Entity, int, string, error) {
	params := url.Values{}
	params.Set("limit", "200")
	if kind != "" && kind != "All" {
		params.Set("filter", "kind="+kind)
	}
	if cursor != "" {
		params.Set("cursor", cursor)
	}
	return c.queryPage(params)
}

// querySearch calls /api/search/query — Backstage's general-purpose search API
// that spans all indexed content (catalog, TechDocs, …).
func (c backstageClient) querySearch(term, cursor string) ([]globalSearchResult, int, string, error) {
	params := url.Values{}
	params.Set("term", term)
	if cursor != "" {
		params.Set("pageCursor", cursor)
	}

	req, err := http.NewRequest("GET", c.baseURL+"/api/search/query?"+params.Encode(), nil)
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

	var result globalSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, 0, "", fmt.Errorf("decoding response: %w", err)
	}
	return result.Results, result.NumberOfResults, result.NextPageCursor, nil
}

// searchEntities performs a full-text search across entity names and titles.
// cursor="" means first page; returns nextCursor="" when no more pages exist.
func (c backstageClient) searchEntities(term, cursor string) ([]Entity, int, string, error) {
	params := url.Values{}
	params.Set("limit", "200")
	params.Set("fullTextFilterTerm", term)
	params.Set("fullTextFilterFields", "metadata.name,metadata.title,metadata.description")
	if cursor != "" {
		params.Set("cursor", cursor)
	}
	return c.queryPage(params)
}
