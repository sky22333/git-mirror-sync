package provider

import (
	"context"

	"git-mirror-sync/internal/model"
)

// Source lists repositories from the origin platform.
type Source interface {
	Name() string
	ListRepositories(ctx context.Context) ([]model.Repository, error)
	AuthUsername() string
	AuthToken() string
}

// Target manages destination repositories and push credentials.
type Target interface {
	Name() string
	Owner() string
	RepositoryExists(ctx context.Context, name string) (bool, error)
	CreateRepository(ctx context.Context, opts model.CreateOptions) error
	// PrepareForMirror makes the target writable for force mirror push
	// (e.g. clear branch/tag protection). No-op when not needed.
	PrepareForMirror(ctx context.Context, name string) error
	GetPushURL(name string) string
	AuthUsername() string
	AuthToken() string
}
