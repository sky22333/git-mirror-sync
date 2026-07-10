package github

import (
	"context"
	"fmt"

	gogithub "github.com/google/go-github/v89/github"

	"git-mirror-sync/internal/config"
	"git-mirror-sync/internal/model"
	"git-mirror-sync/internal/provider"
)

// Source implements provider.Source for GitHub.
type Source struct {
	client          *gogithub.Client
	token           string
	includeForks    bool
	includeArchived bool
}

// NewSource creates a GitHub source provider.
func NewSource(token string, cfg config.SourceConfig) (provider.Source, error) {
	if token == "" {
		return nil, fmt.Errorf("github token is required")
	}
	client, err := gogithub.NewClient(gogithub.WithAuthToken(token))
	if err != nil {
		return nil, fmt.Errorf("create github client: %w", err)
	}
	return &Source{
		client:          client,
		token:           token,
		includeForks:    cfg.ShouldIncludeForks(),
		includeArchived: cfg.ShouldIncludeArchived(),
	}, nil
}

func (s *Source) Name() string          { return "github" }
func (s *Source) AuthUsername() string  { return "x-access-token" }
func (s *Source) AuthToken() string     { return s.token }

// ListRepositories returns all repositories visible to the authenticated user.
func (s *Source) ListRepositories(ctx context.Context) ([]model.Repository, error) {
	opts := &gogithub.RepositoryListByAuthenticatedUserOptions{
		Affiliation: "owner,organization_member",
		ListOptions: gogithub.ListOptions{PerPage: 100},
	}

	var out []model.Repository
	for {
		repos, resp, err := s.client.Repositories.ListByAuthenticatedUser(ctx, opts)
		if err != nil {
			return nil, fmt.Errorf("list github repositories: %w", err)
		}
		for _, r := range repos {
			if r == nil {
				continue
			}
			if !s.includeForks && r.GetFork() {
				continue
			}
			if !s.includeArchived && r.GetArchived() {
				continue
			}
			out = append(out, model.Repository{
				Name:          r.GetName(),
				FullName:      r.GetFullName(),
				Description:   r.GetDescription(),
				Private:       r.GetPrivate(),
				CloneURL:      r.GetCloneURL(),
				HTMLURL:       r.GetHTMLURL(),
				Fork:          r.GetFork(),
				Archived:      r.GetArchived(),
				DefaultBranch: r.GetDefaultBranch(),
			})
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return out, nil
}
