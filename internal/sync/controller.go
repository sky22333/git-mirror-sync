package sync

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"git-mirror-sync/internal/config"
	"git-mirror-sync/internal/mirror"
	"git-mirror-sync/internal/model"
	"git-mirror-sync/internal/provider"
	"git-mirror-sync/internal/report"
)

// Controller orchestrates source listing and multi-target mirroring.
type Controller struct {
	cfg     *config.Config
	source  provider.Source
	targets []provider.Target
	engine  *mirror.Engine
	logger  *slog.Logger
	workDir string
}

// New creates a sync controller.
func New(cfg *config.Config, source provider.Source, targets []provider.Target, logger *slog.Logger) *Controller {
	if logger == nil {
		logger = slog.Default()
	}
	return &Controller{
		cfg:     cfg,
		source:  source,
		targets: targets,
		engine:  mirror.New(),
		logger:  logger,
		workDir: os.TempDir(),
	}
}

// Run executes a full sync cycle and returns a report.
func (c *Controller) Run(ctx context.Context) (*report.Report, error) {
	rep := &report.Report{StartedAt: time.Now()}
	defer func() { rep.Finished = time.Now() }()

	c.logger.Info("listing source repositories", "provider", c.source.Name())
	repos, err := c.source.ListRepositories(ctx)
	if err != nil {
		return rep, fmt.Errorf("list source repositories: %w", err)
	}
	c.logger.Info("discovered repositories", "count", len(repos))

	type job struct {
		repo   model.Repository
		target provider.Target
	}

	jobs := make([]job, 0, len(repos)*len(c.targets))
	for _, repo := range repos {
		for _, target := range c.targets {
			jobs = append(jobs, job{repo: repo, target: target})
		}
	}

	concurrency := c.cfg.Policy.Concurrency
	if concurrency < 1 {
		concurrency = 1
	}

	sem := make(chan struct{}, concurrency)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, j := range jobs {
		wg.Add(1)
		sem <- struct{}{}
		go func(j job) {
			defer wg.Done()
			defer func() { <-sem }()
			item := c.syncOneWithRetry(ctx, j.repo, j.target)
			mu.Lock()
			rep.Add(item)
			mu.Unlock()
		}(j)
	}
	wg.Wait()
	return rep, nil
}

func (c *Controller) syncOneWithRetry(ctx context.Context, repo model.Repository, target provider.Target) report.Item {
	retries := c.cfg.Policy.RetryLimit()
	var item report.Item
	for attempt := 0; attempt <= retries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<(attempt-1)) * time.Second // 1s, 2s, 4s...
			c.logger.Warn("retrying sync after transient error",
				"repo", repo.FullName,
				"target", fmt.Sprintf("%s:%s/%s", target.Name(), target.Owner(), repo.Name),
				"attempt", attempt,
				"max_retries", retries,
				"backoff", backoff,
				"err", item.Message,
			)
			select {
			case <-ctx.Done():
				item.Status = report.StatusFailed
				item.Message = ctx.Err().Error()
				return item
			case <-time.After(backoff):
			}
		}

		item = c.syncOne(ctx, repo, target)
		if item.Status != report.StatusFailed {
			return item
		}
		if attempt == retries || !isTransient(fmt.Errorf("%s", item.Message)) {
			return item
		}
	}
	return item
}

func (c *Controller) syncOne(ctx context.Context, repo model.Repository, target provider.Target) report.Item {
	start := time.Now()
	item := report.Item{
		SourceRepo: repo.FullName,
		Target:     fmt.Sprintf("%s:%s/%s", target.Name(), target.Owner(), repo.Name),
	}

	c.logger.Info("syncing", "repo", repo.FullName, "target", item.Target)

	exists, err := target.RepositoryExists(ctx, repo.Name)
	if err != nil {
		item.Status = report.StatusFailed
		item.Message = err.Error()
		item.Duration = time.Since(start)
		c.logger.Error("check target repo failed", "repo", repo.FullName, "err", err)
		return item
	}

	created := false
	if !exists {
		if !c.cfg.Policy.ShouldCreateMissing() {
			item.Status = report.StatusSkipped
			item.Message = "target missing and create_missing=false"
			item.Duration = time.Since(start)
			return item
		}
		private := c.cfg.VisibilityPolicy().ResolvePrivate(repo.Private)
		err = target.CreateRepository(ctx, model.CreateOptions{
			Name:        repo.Name,
			Description: repo.Description,
			Private:     private,
		})
		if err != nil {
			item.Status = report.StatusFailed
			item.Message = err.Error()
			item.Duration = time.Since(start)
			c.logger.Error("create target repo failed", "repo", repo.FullName, "err", err)
			return item
		}
		created = true
		c.logger.Info("created target repository", "target", item.Target)
	}

	if err = target.PrepareForMirror(ctx, repo.Name); err != nil {
		item.Status = report.StatusFailed
		item.Message = err.Error()
		item.Duration = time.Since(start)
		c.logger.Error("prepare target for mirror failed", "repo", repo.FullName, "err", err)
		return item
	}
	c.logger.Info("prepared target for mirror", "target", item.Target)

	err = c.engine.Sync(ctx, mirror.Options{
		SourceURL: repo.CloneURL,
		TargetURL: target.GetPushURL(repo.Name),
		SourceAuth: mirror.Auth{
			Username: c.source.AuthUsername(),
			Password: c.source.AuthToken(),
		},
		TargetAuth: mirror.Auth{
			Username: target.AuthUsername(),
			Password: target.AuthToken(),
		},
		WorkDir: c.workDir,
	})
	if err != nil {
		item.Status = report.StatusFailed
		item.Message = err.Error()
		item.Duration = time.Since(start)
		c.logger.Error("mirror failed", "repo", repo.FullName, "err", err)
		return item
	}

	if created {
		item.Status = report.StatusCreated
	} else {
		item.Status = report.StatusOK
	}
	item.Duration = time.Since(start)
	c.logger.Info("sync done", "repo", repo.FullName, "target", item.Target, "status", item.Status)
	return item
}
