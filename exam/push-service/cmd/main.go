package main

import (
	"log/slog"
	"net/http"
	"os"

	"push-service/internal/config"
	"push-service/internal/handler"
	"push-service/internal/storage"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg := config.Load()

	db, err := storage.NewDB(cfg.DB)
	if err != nil {
		logger.Error("db connect failed", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	h := handler.New(db, logger)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /notify", h.Notify)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	logger.Info("push-service started", "addr", ":8082")
	if err := http.ListenAndServe(":8082", mux); err != nil {
		logger.Error("server error", "err", err)
	}
}
