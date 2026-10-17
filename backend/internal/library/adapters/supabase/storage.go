// Package supabase implements ports.FileStorage on Supabase Storage.
package supabase

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

// Bucket is the private bucket readers upload their books into.
const Bucket = "books"

type Storage struct {
	http       *http.Client
	baseURL    string // https://<project>.supabase.co
	serviceKey string
}

func NewStorage(supabaseURL, serviceKey string) *Storage {
	return &Storage{http: &http.Client{Timeout: 10 * time.Second}, baseURL: supabaseURL, serviceKey: serviceKey}
}

func (s *Storage) SignedURL(ctx context.Context, path string, ttl time.Duration) (string, error) {
	body, _ := json.Marshal(map[string]int{"expiresIn": int(ttl.Seconds())})
	endpoint := fmt.Sprintf("%s/storage/v1/object/sign/%s/%s", s.baseURL, Bucket, escapePath(path))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+s.serviceKey)
	req.Header.Set("apikey", s.serviceKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.http.Do(req)
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
	return s.baseURL + "/storage/v1" + out.SignedURL, nil
}

func escapePath(p string) string {
	parts := strings.Split(p, "/")
	for i, s := range parts {
		parts[i] = url.PathEscape(s)
	}
	return strings.Join(parts, "/")
}
