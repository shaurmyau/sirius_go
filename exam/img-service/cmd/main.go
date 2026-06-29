package main

import (
	"log/slog"
	"net/http"
	"os"

	"img-service/internal/config"
	"img-service/internal/handler"
	"img-service/internal/queue"
	"img-service/internal/storage"
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

	s3, err := storage.NewS3(cfg.S3)
	if err != nil {
		logger.Error("s3 connect failed", "err", err)
		os.Exit(1)
	}

	q := queue.New(db, s3, cfg, logger)
	go q.Run()

	h := handler.New(q, logger)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /img/process", h.Process)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	logger.Info("img-service started", "addr", ":8081")
	if err := http.ListenAndServe(":8081", mux); err != nil {
		logger.Error("server error", "err", err)
	}
}
