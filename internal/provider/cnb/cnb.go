package cnb

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	cnbsdk "cnb.cool/cnb/sdk/go-cnb/cnb"
	"cnb.cool/cnb/sdk/go-cnb/cnb/types/dto"

	"git-mirror-sync/internal/config"
	"git-mirror-sync/internal/model"
	"git-mirror-sync/internal/provider"
)

// Target implements provider.Target for CNB.
type Target struct {
	client  *cnbsdk.Client
	token   string
	owner   string
	baseURL string
}

// NewTarget creates a CNB target provider.
func NewTarget(token string, cfg config.TargetConfig) (provider.Target, error) {
	if token == "" {
		return nil, fmt.Errorf("cnb token is required")
	}
	if cfg.Owner == "" {
		return nil, fmt.Errorf("cnb owner (organization/group) is required")
	}
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	if baseURL == "" {
		baseURL = "https://cnb.cool"
	}
	apiURL := strings.TrimRight(cfg.APIURL, "/")
	if apiURL == "" {
		apiURL = "https://api.cnb.cool"
	}

	client, err := cnbsdk.NewClient(nil).WithAuthToken(token).WithURLs(apiURL + "/")
	if err != nil {
		return nil, fmt.Errorf("create cnb client: %w", err)
	}
	return &Target{
		client:  client,
		token:   token,
		owner:   strings.Trim(cfg.Owner, "/"),
		baseURL: baseURL,
	}, nil
}

func (t *Target) Name() string         { return "cnb" }
func (t *Target) Owner() string        { return t.owner }
func (t *Target) AuthUsername() string { return "cnb" }
func (t *Target) AuthToken() string    { return t.token }

func (t *Target) slug(name string) string {
	return t.owner + "/" + name
}

// RepositoryExists checks whether the repository exists.
func (t *Target) RepositoryExists(ctx context.Context, name string) (bool, error) {
	_, resp, err := t.client.Repositories.GetByID(ctx, t.slug(name))
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return false, nil
		}
		// Some CNB responses may not populate StatusCode consistently.
		if resp == nil && strings.Contains(strings.ToLower(err.Error()), "404") {
			return false, nil
		}
		return false, fmt.Errorf("get cnb repo %s: %w", t.slug(name), err)
	}
	return true, nil
}

// CreateRepository creates a repository under the configured group/org.
func (t *Target) CreateRepository(ctx context.Context, opts model.CreateOptions) error {
	vis := dto.CreateRepoReqVisibilityPrivate
	if !opts.Private {
		vis = dto.CreateRepoReqVisibilityPublic
	}
	req := &cnbsdk.CreateRepoRequest{
		Name:        opts.Name,
		Description: opts.Description,
		Visibility:  vis,
	}
	resp, err := t.client.Repositories.CreateRepo(ctx, t.owner, req)
	if err != nil {
		return fmt.Errorf("create cnb repo %s/%s: %w", t.owner, opts.Name, err)
	}
	if resp != nil && resp.StatusCode >= 300 {
		return fmt.Errorf("create cnb repo %s/%s: unexpected status %s", t.owner, opts.Name, resp.Status)
	}
	return nil
}

// PrepareForMirror is a no-op: CNB does not protect new repos by default.
func (t *Target) PrepareForMirror(context.Context, string) error { return nil }

// GetPushURL returns the HTTPS push URL.
func (t *Target) GetPushURL(name string) string {
	return fmt.Sprintf("%s/%s/%s.git", t.baseURL, t.owner, name)
}
