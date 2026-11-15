//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// maxErrorBody is how much of an unexpected response body goes into an error.
const maxErrorBody = 300

// supabase calls the local Supabase Auth and Storage APIs. The service role
// and anon keys are only ever sent as headers, never logged.
type supabase struct {
	url        string
	serviceKey string
	anonKey    string
	hc         *http.Client
}

// createUser creates a confirmed user through the auth admin API and
// returns its id.
func (sb *supabase) createUser(ctx context.Context, email, password string) (string, error) {
	body := map[string]any{"email": email, "password": password, "email_confirm": true}
	status, raw, err := sb.send(ctx, http.MethodPost, "/auth/v1/admin/users", sb.serviceKey, body)
	if err != nil {
		return "", err
	}
	if status != http.StatusOK && status != http.StatusCreated {
		return "", fmt.Errorf("create user: HTTP %d %s", status, snippet(raw))
	}
	var u struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &u); err != nil || u.ID == "" {
		return "", fmt.Errorf("create user: no id in the response (%v)", err)
	}
	return u.ID, nil
}

// signIn signs in with the password grant and returns the access token.
func (sb *supabase) signIn(ctx context.Context, email, password string) (string, error) {
	body := map[string]any{"email": email, "password": password}
	status, raw, err := sb.sendAs(ctx, http.MethodPost, "/auth/v1/token?grant_type=password", sb.anonKey, "", body)
	if err != nil {
		return "", err
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("sign in: HTTP %d %s", status, snippet(raw))
	}
	var tok struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(raw, &tok); err != nil || tok.AccessToken == "" {
		return "", fmt.Errorf("sign in: no access_token in the response (%v)", err)
	}
	return tok.AccessToken, nil
}

// deleteUser deletes a user through the auth admin API. A user that is
// already gone is not an error.
func (sb *supabase) deleteUser(ctx context.Context, id string) error {
	status, raw, err := sb.send(ctx, http.MethodDelete, "/auth/v1/admin/users/"+url.PathEscape(id), sb.serviceKey, nil)
	if err != nil {
		return err
	}
	if status != http.StatusOK && status != http.StatusNoContent && status != http.StatusNotFound {
		return fmt.Errorf("delete user %s: HTTP %d %s", id, status, snippet(raw))
	}
	return nil
}

// upload stores data at path in the books bucket with a reader's token and
// returns the status code.
func (sb *supabase) upload(ctx context.Context, token, path string, data []byte) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sb.url+"/storage/v1/object/books/"+path,
		bytes.NewReader(data))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/epub+zip")
	return sb.do(req, sb.anonKey, token)
}

// list returns the names of the objects under prefix in the books bucket,
// as a reader with token sees them.
func (sb *supabase) list(ctx context.Context, token, prefix string) ([]string, error) {
	body := map[string]any{"prefix": prefix, "limit": 100, "offset": 0}
	status, raw, err := sb.sendAs(ctx, http.MethodPost, "/storage/v1/object/list/books", sb.anonKey, token, body)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("list %s: HTTP %d %s", prefix, status, snippet(raw))
	}
	var objs []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(raw, &objs); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(objs))
	for _, o := range objs {
		names = append(names, o.Name)
	}
	return names, nil
}

// deleteObject removes path from the books bucket with the service role.
// An object that is already gone is not an error.
func (sb *supabase) deleteObject(ctx context.Context, path string) error {
	status, raw, err := sb.send(ctx, http.MethodDelete, "/storage/v1/object/books/"+path, sb.serviceKey, nil)
	if err != nil {
		return err
	}
	notFound := status == http.StatusNotFound ||
		(status == http.StatusBadRequest && strings.Contains(string(raw), "not_found"))
	if status != http.StatusOK && !notFound {
		return fmt.Errorf("delete object %s: HTTP %d %s", path, status, snippet(raw))
	}
	return nil
}

// send calls the API with key as both the apikey and the bearer token.
func (sb *supabase) send(ctx context.Context, method, path, key string, body any) (int, []byte, error) {
	return sb.sendAs(ctx, method, path, key, key, body)
}

// sendAs calls the API with apikey and, when set, a bearer token.
func (sb *supabase) sendAs(ctx context.Context, method, path, apikey, bearer string, body any) (int, []byte, error) {
	var rd io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return 0, nil, err
		}
		rd = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, sb.url+path, rd)
	if err != nil {
		return 0, nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	status, raw, err := sb.do(req, apikey, bearer)
	return status, raw, err
}

func (sb *supabase) do(req *http.Request, apikey, bearer string) (int, []byte, error) {
	req.Header.Set("apikey", apikey)
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := sb.hc.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("%s %s: %w", req.Method, req.URL.Path, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	return resp.StatusCode, raw, err
}

// snippet shortens a response body for an error message.
func snippet(raw []byte) string {
	s := strings.TrimSpace(string(raw))
	if len(s) > maxErrorBody {
		s = s[:maxErrorBody] + "…"
	}
	return s
}
