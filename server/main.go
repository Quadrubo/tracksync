package main

import (
	"context"
	"database/sql"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/Quadrubo/tracksync/server/internal/config"
	"github.com/Quadrubo/tracksync/server/internal/server"
	"github.com/Quadrubo/tracksync/server/internal/target"
	_ "github.com/Quadrubo/tracksync/server/internal/target/dawarich"
	_ "modernc.org/sqlite"
)

func main() {
	envFile := flag.String("env-file", "", "path to .env config file (default: read from environment)")
	flag.Parse()

	cfg, err := config.Load(*envFile)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// Build targets from account config
	targets := make(map[string]target.Target)
	for _, account := range cfg.Accounts {
		t, err := target.Get(account.TargetType, target.Config{
			URL:        account.TargetURL,
			APIKey:     account.APIKey,
			APIKeyFile: account.APIKeyFile,
			Timeout:    cfg.TargetTimeout,
		})
		if err != nil {
			slog.Error("failed to create target", "device", account.DeviceID, "error", err)
			os.Exit(1)
		}
		targets[account.DeviceID] = t
		slog.Info("configured target", "device", account.DeviceID, "type", account.TargetType, "url", account.TargetURL)
	}

	if err := os.MkdirAll(filepath.Dir(cfg.StateDB), 0755); err != nil {
		slog.Error("failed to create data directory", "error", err)
		os.Exit(1)
	}

	db, err := sql.Open("sqlite", cfg.StateDB)
	if err != nil {
		slog.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer func() { _ = db.Close() }()

	if err := server.InitDB(db); err != nil {
		slog.Error("failed to init database", "error", err)
		os.Exit(1)
	}

	srv := server.New(cfg, db, targets)

	addr := ":" + cfg.Port
	httpServer := &http.Server{
		Addr:              addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigCh
		slog.Info("received signal, shutting down", "signal", sig)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(ctx); err != nil {
			slog.Error("shutdown error", "error", err)
		}
	}()

	slog.Info("listening", "addr", addr)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}
