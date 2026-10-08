// Package storage signs short-lived download URLs for readers' private books.
package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const Bucket = "books"

type Client struct {
	http       *http.Client
	baseURL    string // https://<project>.supabase.co
	serviceKey string
}

func New(supabaseURL, serviceKey string) *Client {
	return &Client{http: &http.Client{Timeout: 10 * time.Second}, baseURL: supabaseURL, serviceKey: serviceKey}
}

// SignedURL returns a URL that can download the object for ttl.
func (c *Client) SignedURL(ctx context.Context, path string, ttl time.Duration) (string, error) {
	body, _ := json.Marshal(map[string]int{"expiresIn": int(ttl.Seconds())})
	endpoint := fmt.Sprintf("%s/storage/v1/object/sign/%s/%s", c.baseURL, Bucket, escapePath(path))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.serviceKey)
	req.Header.Set("apikey", c.serviceKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("sign url: %s", resp.Status)
	}
	var out struct {
		SignedURL string `json:"signedURL"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if out.SignedURL == "" {
		return "", fmt.Errorf("sign url: empty response")
	}
	return c.baseURL + "/storage/v1" + out.SignedURL, nil
}

func escapePath(p string) string {
	parts := strings.Split(p, "/")
	for i, s := range parts {
		parts[i] = url.PathEscape(s)
	}
	return strings.Join(parts, "/")
}
