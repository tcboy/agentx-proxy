package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/agentx-labs/agentx-proxy/internal/config"
	"github.com/agentx-labs/agentx-proxy/internal/mysql"
	"github.com/agentx-labs/agentx-proxy/internal/proxy/clickhouse"
	"github.com/agentx-labs/agentx-proxy/internal/proxy/postgresql"
)

func main() {
	cfgPath := "config.yaml"
	if v := os.Getenv("CONFIG_PATH"); v != "" {
		cfgPath = v
	}

	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	setupLogger(cfg)

	slog.Info("starting agentx-proxy",
		"pg_listen", cfg.Listen.PostgreSQL,
		"ch_native_listen", cfg.Listen.ClickHouseNative,
		"ch_http_listen", cfg.Listen.ClickHouseHTTP,
		"mysql_host", cfg.MySQL.Host,
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize MySQL connection pool
	pool, err := mysql.NewPool(&cfg.MySQL)
	if err != nil {
		slog.Error("failed to create MySQL pool", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	// Auto-initialize schema
	if err := mysql.EnsureSchema(pool, cfg); err != nil {
		slog.Error("failed to ensure schema", "error", err)
		os.Exit(1)
	}

	// Start PG proxy
	var pgServer *postgresql.Server
	if cfg.Proxy.PGToMySQL.Enabled {
		pgServer, err = postgresql.NewServer(cfg, pool)
		if err != nil {
			slog.Error("failed to create PG server", "error", err)
			os.Exit(1)
		}
		go func() {
			if err := pgServer.Start(ctx); err != nil {
				slog.Error("PG server error", "error", err)
			}
		}()
		slog.Info("PostgreSQL proxy started", "addr", cfg.Listen.PostgreSQL)
	}

	// Start CH Native proxy
	var chNativeServer *clickhouse.NativeServer
	if cfg.Proxy.CHToMySQL.Enabled {
		chNativeServer, err = clickhouse.NewNativeServer(cfg, pool)
		if err != nil {
			slog.Error("failed to create CH native server", "error", err)
			os.Exit(1)
		}
		go func() {
			if err := chNativeServer.Start(ctx); err != nil {
				slog.Error("CH native server error", "error", err)
			}
		}()
		slog.Info("ClickHouse Native proxy started", "addr", cfg.Listen.ClickHouseNative)
	}

	// Start CH HTTP proxy
	var chHTTPServer *clickhouse.HTTPServer
	if cfg.Proxy.CHToMySQL.Enabled {
		chHTTPServer, err = clickhouse.NewHTTPServer(cfg, pool)
		if err != nil {
			slog.Error("failed to create CH HTTP server", "error", err)
			os.Exit(1)
		}
		go func() {
			if err := chHTTPServer.Start(ctx); err != nil {
				slog.Error("CH HTTP server error", "error", err)
			}
		}()
		slog.Info("ClickHouse HTTP proxy started", "addr", cfg.Listen.ClickHouseHTTP)
	}

	slog.Info("agentx-proxy is ready")

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	slog.Info("shutting down...")
	cancel()

	if pgServer != nil {
		pgServer.Close()
	}
	if chNativeServer != nil {
		chNativeServer.Close()
	}
	if chHTTPServer != nil {
		chHTTPServer.Close()
	}

	slog.Info("agentx-proxy stopped")
}

func setupLogger(cfg *config.Config) {
	level := slog.LevelInfo
	switch cfg.Log.Level {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}

	opts := &slog.HandlerOptions{Level: level}
	var handler slog.Handler

	if cfg.Log.Format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	slog.SetDefault(slog.New(handler))
}

func init() {
	// Ensure we print full stack traces for debugging
	if os.Getenv("AGENTX_DEBUG") == "1" {
		fmt.Fprintln(os.Stderr, "agentx-proxy debug mode enabled")
	}
}
