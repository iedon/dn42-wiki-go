package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/iedon/dn42-wiki-go/config"
	"github.com/iedon/dn42-wiki-go/gitutil"
	"github.com/iedon/dn42-wiki-go/server"
	"github.com/iedon/dn42-wiki-go/site"
	"github.com/iedon/dn42-wiki-go/templatex"
)

func main() {
	cfgPath := flag.String("config", "config.json", "path to configuration file")
	buildFlag := flag.Bool("build", false, "force static build mode")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		panic(err)
	}

	if *buildFlag {
		cfg.Live = false
	}

	logger := newLogger(cfg.LogLevel)
	logger.Info("starting", "live", cfg.Live)

	repo, err := gitutil.NewRepository(cfg.Git.BinPath, cfg.Git.Remote, cfg.Git.LocalDirectory)
	if err != nil {
		logger.Error("repository", "error", err)
		os.Exit(1)
	}

	templates, err := templatex.Load(cfg.TemplateDir)
	if err != nil {
		logger.Error("templates", "error", err)
		os.Exit(1)
	}

	svc := site.NewService(cfg, repo, templates)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// not live mode, live=false or run with --build flag
	if !cfg.Live {
		if err := svc.BuildStatic(ctx); err != nil {
			logger.Error("build", "error", err)
			os.Exit(1)
		}
		logger.Info("static build completed", "output", cfg.OutputDir)
		return
	}

	go pullLoop(ctx, svc, cfg.PullInterval, logger)

	srv := server.New(cfg, svc, logger, SERVER_HEADER)
	if err := srv.Start(ctx); err != nil {
		logger.Error("server", "error", err)
		os.Exit(1)
	}
}

func pullLoop(ctx context.Context, svc *site.Service, interval time.Duration, logger *slog.Logger) {
	if interval <= 0 {
		return
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := svc.Pull(ctx); err != nil {
				logger.Warn("pull", "error", err)
			}
		}
	}
}

func newLogger(level string) *slog.Logger {
	var lvl slog.Level
	switch strings.ToLower(level) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: lvl}))
}
