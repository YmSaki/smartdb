package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"smartdb/internal/api"
	"smartdb/internal/auth"
	"smartdb/internal/config"
	"smartdb/internal/domain"
	"smartdb/internal/handler"
	"time"
)

var systemDB = "./system.db"

func main() {
	cfg := config.Load()

	logLevel := slog.LevelInfo
	switch cfg.LogLevel {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	}

	slogHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})
	logger := slog.New(slogHandler)
	slog.SetDefault(logger)

	db, err := InitializeSystemDB(systemDB)
	if err != nil {
		slog.Error("Failed to initialize system database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	app := &domain.App{
		SystemDB: db,
		Config:   cfg,
	}

	if err := auth.BootstrapAdminKey(db); err != nil {
		slog.Error("Failed to bootstrap admin key", "error", err)
		os.Exit(1)
	}

	serverMux := http.NewServeMux()
	serverMux.Handle(
		"/api/",
		http.StripPrefix("/api", api.RouterMux(app)),
	)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      handler.RequestIDMiddleware(serverMux),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	go func() {
		slog.Info("Server starting", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server failed", "error", err)
		}
	}()

	<-ctx.Done()
	slog.Info("Shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("Server shutdown failed", "error", err)
	}

	slog.Info("Server stopped")
}
