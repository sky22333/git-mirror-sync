package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"git-mirror-sync/internal/config"
	"git-mirror-sync/internal/provider"
	cnbprovider "git-mirror-sync/internal/provider/cnb"
	codebergprovider "git-mirror-sync/internal/provider/codeberg"
	giteeprovider "git-mirror-sync/internal/provider/gitee"
	githubprovider "git-mirror-sync/internal/provider/github"
	gitlabprovider "git-mirror-sync/internal/provider/gitlab"
	syncctrl "git-mirror-sync/internal/sync"
)

// 由 -ldflags "-X main.version=..." 注入，本地构建默认为 dev。
var version = "dev"

func main() {
	configPath := flag.String("config", "config.toml", "path to config.toml")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		return
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	logger.Info("git-mirror-sync", "version", version)

	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Error("load config failed", "err", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	source, err := githubprovider.NewSource(mustToken(cfg.Source.TokenEnv), cfg.Source)
	if err != nil {
		logger.Error("init source failed", "err", err)
		os.Exit(1)
	}

	targets := make([]provider.Target, 0, len(cfg.Targets))
	for _, tcfg := range cfg.Targets {
		token, err := config.TokenFromEnv(tcfg.TokenEnv)
		if err != nil {
			logger.Error("missing target token", "provider", tcfg.Provider, "err", err)
			os.Exit(1)
		}
		var target provider.Target
		switch tcfg.Provider {
		case "gitlab":
			target, err = gitlabprovider.NewTarget(token, tcfg)
		case "gitee":
			target, err = giteeprovider.NewTarget(token, tcfg)
		case "cnb":
			target, err = cnbprovider.NewTarget(token, tcfg)
		case "codeberg":
			target, err = codebergprovider.NewTarget(token, tcfg)
		default:
			err = fmt.Errorf("unsupported provider %q", tcfg.Provider)
		}
		if err != nil {
			logger.Error("init target failed", "provider", tcfg.Provider, "err", err)
			os.Exit(1)
		}
		targets = append(targets, target)
		logger.Info("target ready", "provider", target.Name(), "owner", target.Owner())
	}

	ctrl := syncctrl.New(cfg, source, targets, logger)
	rep, err := ctrl.Run(ctx)
	if rep != nil {
		rep.Write(os.Stdout)
	}
	if err != nil {
		logger.Error("sync failed", "err", err)
		os.Exit(1)
	}
	if rep != nil && rep.Failed() {
		os.Exit(1)
	}
}

func mustToken(envName string) string {
	token, err := config.TokenFromEnv(envName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	return token
}
