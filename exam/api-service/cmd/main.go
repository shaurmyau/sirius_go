package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"api-service/internal/config"
	"api-service/internal/handler"
	"api-service/internal/middleware"
	"api-service/internal/storage"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/prometheus/client_golang/prometheus/promhttp"
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

	jwks, err := keyfunc.NewDefaultCtx(context.Background(), []string{cfg.JWKSUrl})
	if err != nil {
		logger.Error("jwks init failed", "err", err)
		os.Exit(1)
	}

	auth := middleware.NewAuth(jwks)
	h := handler.New(db, s3, cfg, logger)

	mux := http.NewServeMux()
	mux.Handle("POST /api/img", auth.Middleware(http.HandlerFunc(h.PostImage)))
	mux.Handle("GET /api/img", auth.Middleware(http.HandlerFunc(h.GetImage)))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	mux.Handle("GET /metrics", promhttp.Handler())

	logger.Info("api-service started", "addr", ":8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		logger.Error("server error", "err", err)
	}
}
