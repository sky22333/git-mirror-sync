package gitlab

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"

	"git-mirror-sync/internal/config"
	"git-mirror-sync/internal/model"
	"git-mirror-sync/internal/provider"
)

// Target implements provider.Target for GitLab.
type Target struct {
	client  *gitlab.Client
	token   string
	owner   string
	baseURL string
}

// NewTarget creates a GitLab target provider.
func NewTarget(token string, cfg config.TargetConfig) (provider.Target, error) {
	if token == "" {
		return nil, fmt.Errorf("gitlab token is required")
	}
	if cfg.Owner == "" {
		return nil, fmt.Errorf("gitlab owner is required")
	}
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	if baseURL == "" {
		baseURL = "https://gitlab.com"
	}

	client, err := gitlab.NewClient(token, gitlab.WithBaseURL(baseURL+"/api/v4"))
	if err != nil {
		return nil, fmt.Errorf("create gitlab client: %w", err)
	}
	return &Target{
		client:  client,
		token:   token,
		owner:   cfg.Owner,
		baseURL: baseURL,
	}, nil
}

func (t *Target) Name() string         { return "gitlab" }
func (t *Target) Owner() string        { return t.owner }
func (t *Target) AuthUsername() string { return "oauth2" }
func (t *Target) AuthToken() string    { return t.token }

func (t *Target) projectPath(name string) string {
	return t.owner + "/" + name
}

// RepositoryExists checks whether the project already exists.
func (t *Target) RepositoryExists(ctx context.Context, name string) (bool, error) {
	_, resp, err := t.client.Projects.GetProject(t.projectPath(name), nil, gitlab.WithContext(ctx))
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return false, nil
		}
		var errResp *gitlab.ErrorResponse
		if errors.As(err, &errResp) && errResp.Response != nil && errResp.Response.StatusCode == http.StatusNotFound {
			return false, nil
		}
		return false, fmt.Errorf("get gitlab project %s: %w", t.projectPath(name), err)
	}
	return true, nil
}

// CreateRepository creates a new GitLab project under the configured owner.
func (t *Target) CreateRepository(ctx context.Context, opts model.CreateOptions) error {
	visibility := gitlab.PrivateVisibility
	if !opts.Private {
		visibility = gitlab.PublicVisibility
	}

	createOpts := &gitlab.CreateProjectOptions{
		Name:                 gitlab.Ptr(opts.Name),
		Path:                 gitlab.Ptr(opts.Name),
		Description:          gitlab.Ptr(opts.Description),
		Visibility:           gitlab.Ptr(visibility),
		InitializeWithReadme: gitlab.Ptr(false),
	}

	ns, resp, err := t.client.Namespaces.GetNamespace(t.owner, gitlab.WithContext(ctx))
	if err == nil && ns != nil {
		createOpts.NamespaceID = gitlab.Ptr(ns.ID)
	} else if resp != nil && resp.StatusCode != http.StatusNotFound && err != nil {
		return fmt.Errorf("resolve gitlab namespace %s: %w", t.owner, err)
	}

	_, _, err = t.client.Projects.CreateProject(createOpts, gitlab.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("create gitlab project %s/%s: %w", t.owner, opts.Name, err)
	}
	return nil
}

// PrepareForMirror enables force-push on protected branches and clears protected tags.
// Needed when re-syncing existing repos (non-fast-forward mirror updates).
func (t *Target) PrepareForMirror(ctx context.Context, name string) error {
	pid := t.projectPath(name)

	opt := &gitlab.ListProtectedBranchesOptions{
		ListOptions: gitlab.ListOptions{PerPage: 100},
	}
	seen := make(map[string]struct{})
	for {
		branches, resp, err := t.client.ProtectedBranches.ListProtectedBranches(pid, opt, gitlab.WithContext(ctx))
		if err != nil {
			return fmt.Errorf("list protected branches %s: %w", pid, err)
		}
		for _, b := range branches {
			if b == nil || b.Name == "" {
				continue
			}
			seen[b.Name] = struct{}{}
			if err := t.ensureBranchForcePush(ctx, pid, b.Name); err != nil {
				return err
			}
		}
		if resp == nil || resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	// Cover common defaults when only group-level protection applies.
	for _, branch := range []string{"main", "master"} {
		if _, ok := seen[branch]; ok {
			continue
		}
		if err := t.ensureBranchForcePush(ctx, pid, branch); err != nil {
			return err
		}
	}

	tagOpt := &gitlab.ListProtectedTagsOptions{
		ListOptions: gitlab.ListOptions{PerPage: 100},
	}
	for {
		tags, resp, err := t.client.ProtectedTags.ListProtectedTags(pid, tagOpt, gitlab.WithContext(ctx))
		if err != nil {
			return fmt.Errorf("list protected tags %s: %w", pid, err)
		}
		for _, tag := range tags {
			if tag == nil || tag.Name == "" {
				continue
			}
			if _, err := t.client.ProtectedTags.UnprotectRepositoryTags(pid, tag.Name, gitlab.WithContext(ctx)); err != nil {
				return fmt.Errorf("unprotect tag %s on %s: %w", tag.Name, pid, err)
			}
		}
		if resp == nil || resp.NextPage == 0 {
			break
		}
		tagOpt.Page = resp.NextPage
	}
	return nil
}

func (t *Target) ensureBranchForcePush(ctx context.Context, pid, branch string) error {
	pb, resp, err := t.client.ProtectedBranches.GetProtectedBranch(pid, branch, gitlab.WithContext(ctx))
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			_, _, err = t.client.ProtectedBranches.ProtectRepositoryBranches(pid, &gitlab.ProtectRepositoryBranchesOptions{
				Name:            gitlab.Ptr(branch),
				AllowForcePush:  gitlab.Ptr(true),
				PushAccessLevel: gitlab.Ptr(gitlab.MaintainerPermissions),
			}, gitlab.WithContext(ctx))
			if err != nil {
				var errResp *gitlab.ErrorResponse
				if errors.As(err, &errResp) && errResp.Response != nil {
					code := errResp.Response.StatusCode
					if code == http.StatusNotFound || code == http.StatusBadRequest {
						return nil
					}
				}
				return fmt.Errorf("protect branch %s on %s with force push: %w", branch, pid, err)
			}
			return nil
		}
		return fmt.Errorf("get protected branch %s on %s: %w", branch, pid, err)
	}
	if pb != nil && pb.AllowForcePush {
		return nil
	}

	_, _, err = t.client.ProtectedBranches.UpdateProtectedBranch(pid, branch, &gitlab.UpdateProtectedBranchOptions{
		AllowForcePush: gitlab.Ptr(true),
	}, gitlab.WithContext(ctx))
	if err == nil {
		return nil
	}

	if _, uerr := t.client.ProtectedBranches.UnprotectRepositoryBranches(pid, branch, gitlab.WithContext(ctx)); uerr != nil {
		return fmt.Errorf("enable force push on %s/%s: update: %w; unprotect: %v", pid, branch, err, uerr)
	}
	_, _, err = t.client.ProtectedBranches.ProtectRepositoryBranches(pid, &gitlab.ProtectRepositoryBranchesOptions{
		Name:            gitlab.Ptr(branch),
		AllowForcePush:  gitlab.Ptr(true),
		PushAccessLevel: gitlab.Ptr(gitlab.MaintainerPermissions),
	}, gitlab.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("reprotect branch %s on %s with force push: %w", branch, pid, err)
	}
	return nil
}

// GetPushURL returns the HTTPS push URL for the project.
func (t *Target) GetPushURL(name string) string {
	return fmt.Sprintf("%s/%s/%s.git", t.baseURL, t.owner, name)
}
