package main

import (
	"log/slog"
	"net/http"
	"os"
	"smartdb/internal/api"
	"smartdb/internal/domain"
	"time"
)

var systemDB = "./system.db"

func main() {
	handler := slog.NewJSONHandler(os.Stdout, nil)

	logger := slog.New(handler)
	slog.SetDefault(logger)

	db, err := InitializeSystemDB(systemDB)
	if err != nil {
		slog.Error("Failed to initialize system database", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	app := &domain.App{SystemDB: db}

	serverMux := http.NewServeMux()

	serverMux.Handle(
		"/api/",
		http.StripPrefix("/api", api.RouterMux(app)),
	)

	srv := &http.Server{
		Addr:    ":8080",
		Handler: serverMux,

		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("Server failed to start", "error", err)
	}
}
