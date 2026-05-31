package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func FetchModels(ctx context.Context, client *http.Client, provider, baseURL, apiKey string) ([]string, error) {
	provider = strings.TrimSpace(strings.ToLower(provider))
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	apiKey = strings.TrimSpace(apiKey)
	if provider == "" || baseURL == "" || apiKey == "" {
		return nil, fmt.Errorf("provider, base_url and api_key are all required")
	}
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	switch provider {
	case "google":
		return fetchGeminiModels(ctx, client, baseURL, apiKey)
	case "anthropic":
		return fetchListModels(ctx, client, modelsURL(baseURL), func(r *http.Request) {
			r.Header.Set("x-api-key", apiKey)
			r.Header.Set("anthropic-version", "2023-06-01")
		})
	default:
		return fetchListModels(ctx, client, modelsURL(baseURL), func(r *http.Request) {
			r.Header.Set("Authorization", "Bearer "+apiKey)
		})
	}
}

func fetchListModels(ctx context.Context, client *http.Client, endpoint string, auth func(*http.Request)) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	auth(req)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, limitedBody(resp, 512))
	}
	var payload struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	out := make([]string, 0, len(payload.Data))
	for _, m := range payload.Data {
		if id := strings.TrimSpace(m.ID); id != "" {
			out = append(out, id)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("model list is empty")
	}
	return out, nil
}

func fetchGeminiModels(ctx context.Context, client *http.Client, baseURL, apiKey string) ([]string, error) {
	endpoint := baseURL + "/v1beta/models?key=" + url.QueryEscape(apiKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, limitedBody(resp, 512))
	}
	var payload struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	out := make([]string, 0, len(payload.Models))
	for _, m := range payload.Models {
		if name := strings.TrimSpace(strings.TrimPrefix(m.Name, "models/")); name != "" {
			out = append(out, name)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("model list is empty")
	}
	return out, nil
}

func limitedBody(resp *http.Response, max int) string {
	if resp.Body == nil {
		return ""
	}
	data, _ := io.ReadAll(io.LimitReader(resp.Body, int64(max)))
	return strings.TrimSpace(string(data))
}
