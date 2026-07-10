package codeberg

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"git-mirror-sync/internal/config"
	"git-mirror-sync/internal/model"
	"git-mirror-sync/internal/provider"
)

// Target implements provider.Target for Codeberg (Forgejo API v1).
// Self-hosted Forgejo works by overriding base_url / api_url.
type Target struct {
	httpClient *http.Client
	token      string
	owner      string
	baseURL    string
	apiURL     string
	username   string
}

// NewTarget creates a Codeberg/Forgejo target provider.
func NewTarget(token string, cfg config.TargetConfig) (provider.Target, error) {
	if token == "" {
		return nil, fmt.Errorf("codeberg token is required")
	}
	if cfg.Owner == "" {
		return nil, fmt.Errorf("codeberg owner is required")
	}
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	if baseURL == "" {
		baseURL = "https://codeberg.org"
	}
	apiURL := strings.TrimRight(cfg.APIURL, "/")
	if apiURL == "" {
		apiURL = "https://codeberg.org/api/v1"
	}

	t := &Target{
		httpClient: &http.Client{Timeout: 60 * time.Second},
		token:      token,
		owner:      cfg.Owner,
		baseURL:    baseURL,
		apiURL:     apiURL,
	}

	user, err := t.fetchAuthenticatedUser(context.Background())
	if err != nil {
		return nil, fmt.Errorf("verify codeberg token: %w", err)
	}
	t.username = user
	return t, nil
}

func (t *Target) Name() string         { return "codeberg" }
func (t *Target) Owner() string        { return t.owner }
func (t *Target) AuthUsername() string { return t.username }
func (t *Target) AuthToken() string    { return t.token }

type forgejoUser struct {
	Login string `json:"login"`
}

type forgejoRepo struct {
	Name     string `json:"name"`
	HTMLURL  string `json:"html_url"`
	Private  bool   `json:"private"`
	FullName string `json:"full_name"`
}

func (t *Target) fetchAuthenticatedUser(ctx context.Context) (string, error) {
	var user forgejoUser
	if err := t.doJSON(ctx, http.MethodGet, "/user", nil, &user); err != nil {
		return "", err
	}
	if user.Login == "" {
		return "", fmt.Errorf("empty codeberg username")
	}
	return user.Login, nil
}

// RepositoryExists checks whether the repository exists under the owner.
func (t *Target) RepositoryExists(ctx context.Context, name string) (bool, error) {
	path := fmt.Sprintf("/repos/%s/%s", url.PathEscape(t.owner), url.PathEscape(name))
	req, err := t.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return false, err
	}
	resp, err := t.httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	switch resp.StatusCode {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	default:
		return false, fmt.Errorf("codeberg get repo %s/%s: %s: %s", t.owner, name, resp.Status, strings.TrimSpace(string(body)))
	}
}

// CreateRepository creates a user or organization repository.
func (t *Target) CreateRepository(ctx context.Context, opts model.CreateOptions) error {
	payload := map[string]any{
		"name":        opts.Name,
		"description": opts.Description,
		"private":     opts.Private,
		"auto_init":   false,
	}

	var path string
	if strings.EqualFold(t.owner, t.username) {
		path = "/user/repos"
	} else {
		path = fmt.Sprintf("/orgs/%s/repos", url.PathEscape(t.owner))
	}

	var created forgejoRepo
	if err := t.doJSON(ctx, http.MethodPost, path, payload, &created); err != nil {
		return fmt.Errorf("create codeberg repo %s/%s: %w", t.owner, opts.Name, err)
	}
	return nil
}

// PrepareForMirror is a no-op: new Forgejo repos are unprotected by default.
func (t *Target) PrepareForMirror(context.Context, string) error { return nil }

// GetPushURL returns the HTTPS push URL.
func (t *Target) GetPushURL(name string) string {
	return fmt.Sprintf("%s/%s/%s.git", t.baseURL, t.owner, name)
}

func (t *Target) doJSON(ctx context.Context, method, path string, body any, out any) error {
	req, err := t.newRequest(ctx, method, path, body)
	if err != nil {
		return err
	}
	resp, err := t.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%s %s: %s: %s", method, path, resp.Status, strings.TrimSpace(string(data)))
	}
	if out == nil || len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func (t *Target) newRequest(ctx context.Context, method, path string, body any) (*http.Request, error) {
	u := t.apiURL + path
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "token "+t.token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}
