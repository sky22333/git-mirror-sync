package mirror

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/storage/memory"
)

// Auth holds HTTP basic credentials for Git over HTTPS.
type Auth struct {
	Username string
	Password string
}

func (a Auth) method() transport.AuthMethod {
	if a.Username == "" && a.Password == "" {
		return nil
	}
	user := a.Username
	if user == "" {
		user = "git"
	}
	return &http.BasicAuth{Username: user, Password: a.Password}
}

// Options controls a mirror sync operation.
type Options struct {
	SourceURL  string
	TargetURL  string
	SourceAuth Auth
	TargetAuth Auth
	// WorkDir, when set, uses a temporary bare repo on disk under WorkDir.
	// When empty, an in-memory storage is used (fine for small/medium repos).
	WorkDir string
}

// Engine performs pure-Go git mirror clone + push.
type Engine struct{}

// New returns a new mirror engine.
func New() *Engine { return &Engine{} }

// backupRefSpecs mirrors branches and tags only (excludes refs/pull/* etc.).
var backupRefSpecs = []config.RefSpec{
	"+refs/heads/*:refs/heads/*",
	"+refs/tags/*:refs/tags/*",
}

// Sync mirrors all refs from source to target.
func (e *Engine) Sync(ctx context.Context, opts Options) error {
	if opts.SourceURL == "" || opts.TargetURL == "" {
		return fmt.Errorf("source and target URLs are required")
	}

	if opts.WorkDir != "" {
		return e.syncOnDisk(ctx, opts)
	}
	return e.syncInMemory(ctx, opts)
}

func (e *Engine) syncInMemory(ctx context.Context, opts Options) error {
	repo, err := git.CloneContext(ctx, memory.NewStorage(), nil, &git.CloneOptions{
		URL:    opts.SourceURL,
		Auth:   opts.SourceAuth.method(),
		Mirror: true,
	})
	if err != nil {
		if errors.Is(err, transport.ErrEmptyRemoteRepository) {
			return nil
		}
		return fmt.Errorf("mirror clone %s: %w", opts.SourceURL, err)
	}
	return e.pushMirror(ctx, repo, opts)
}

func (e *Engine) syncOnDisk(ctx context.Context, opts Options) error {
	dir, err := os.MkdirTemp(opts.WorkDir, "git-mirror-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(dir)

	repoPath := filepath.Join(dir, "repo.git")
	repo, err := git.PlainCloneContext(ctx, repoPath, true, &git.CloneOptions{
		URL:    opts.SourceURL,
		Auth:   opts.SourceAuth.method(),
		Mirror: true,
	})
	if err != nil {
		if errors.Is(err, transport.ErrEmptyRemoteRepository) {
			return nil
		}
		return fmt.Errorf("mirror clone %s: %w", opts.SourceURL, err)
	}
	return e.pushMirror(ctx, repo, opts)
}

func (e *Engine) pushMirror(ctx context.Context, repo *git.Repository, opts Options) error {
	const remoteName = "mirror-target"
	_, err := repo.CreateRemote(&config.RemoteConfig{
		Name:   remoteName,
		URLs:   []string{opts.TargetURL},
		Mirror: true,
	})
	if err != nil && !errors.Is(err, git.ErrRemoteExists) {
		return fmt.Errorf("create remote: %w", err)
	}

	// Do not prune: GitLab rejects mirror pushes that delete refs when the
	// source/target default-branch sets differ (pre-receive hook declined).
	err = repo.PushContext(ctx, &git.PushOptions{
		RemoteName: remoteName,
		Auth:       opts.TargetAuth.method(),
		RefSpecs:   backupRefSpecs,
		Force:      true,
	})
	if err != nil {
		if errors.Is(err, git.NoErrAlreadyUpToDate) {
			return nil
		}
		if errors.Is(err, transport.ErrEmptyRemoteRepository) {
			return nil
		}
		return fmt.Errorf("mirror push %s: %w", opts.TargetURL, err)
	}
	return nil
}
