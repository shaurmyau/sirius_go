package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"api-service/internal/config"
	"api-service/internal/middleware"
	"api-service/internal/model"
	"api-service/internal/storage"

	"api-service/internal/metrics"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Handler struct {
	db     *storage.DB
	s3     *storage.S3
	cfg    *config.Config
	logger *slog.Logger
}

func New(db *storage.DB, s3 *storage.S3, cfg *config.Config, logger *slog.Logger) *Handler {
	return &Handler{db: db, s3: s3, cfg: cfg, logger: logger}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func (h *Handler) PostImage(w http.ResponseWriter, r *http.Request) {
	sub, ok := middleware.SubFromClaims(r)
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"status": "error", "message": "missing sub"})
		return
	}

	var req struct {
		Name string `json:"name"`
		Type string `json:"type"`
		Size int64  `json:"size"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Warn("bad request body", "err", err)
		writeJSON(w, http.StatusBadRequest, map[string]string{"status": "error", "message": "invalid body"})
		metrics.ReqTotal.WithLabelValues("POST", "/api/img", "400").Inc()
		return
	}

	if !strings.HasPrefix(req.Type, "image/") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"status": "error", "message": "invalid mime type"})
		metrics.ReqTotal.WithLabelValues("POST", "/api/img", "400").Inc()
		return
	}
	if req.Size > 10*1024*1024 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"status": "error", "message": "file too large"})
		metrics.ReqTotal.WithLabelValues("POST", "/api/img", "400").Inc()
		return
	}
	if len(req.Name) > 255 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"status": "error", "message": "name too long"})
		metrics.ReqTotal.WithLabelValues("POST", "/api/img", "400").Inc()
		return
	}

	userID, err := uuid.Parse(sub)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"status": "error", "message": "invalid sub"})
		return
	}

	objectID := uuid.New()
	presigned, err := h.s3.PresignedPutURL(r.Context(), objectID.String())
	if err != nil {
		h.logger.Error("presign failed", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"status": "error", "message": "s3 error"})
		metrics.ReqTotal.WithLabelValues("POST", "/api/img", "500").Inc()
		return
	}

	img := &model.Image{
		ID:       objectID,
		UserID:   userID,
		Name:     req.Name,
		MimeType: req.Type,
		Size:     req.Size,
		Status:   "upload",
	}
	if err := h.db.CreateImage(r.Context(), img); err != nil {
		h.logger.Error("db insert failed", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"status": "error", "message": "db error"})
		metrics.ReqTotal.WithLabelValues("POST", "/api/img", "500").Inc()
		return
	}

	h.logger.Info("image created", "id", objectID, "user", sub)
	metrics.ReqTotal.WithLabelValues("POST", "/api/img", "200").Inc()
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"data": map[string]string{
			"object_id": objectID.String(),
			"endpoint":  presigned.String(),
		},
	})
}

func (h *Handler) GetImage(w http.ResponseWriter, r *http.Request) {
	sub, ok := middleware.SubFromClaims(r)
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"status": "error", "message": "missing sub"})
		return
	}

	idStr := r.URL.Query().Get("id")
	imageID, err := uuid.Parse(idStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"status": "error", "message": "invalid id"})
		metrics.ReqTotal.WithLabelValues("GET", "/api/img", "400").Inc()
		return
	}

	img, err := h.db.GetImage(r.Context(), imageID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"status": "error", "message": "not found"})
			metrics.ReqTotal.WithLabelValues("GET", "/api/img", "404").Inc()
			return
		}
		h.logger.Error("db get failed", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"status": "error", "message": "db error"})
		metrics.ReqTotal.WithLabelValues("GET", "/api/img", "500").Inc()
		return
	}

	if img.UserID.String() != sub {
		writeJSON(w, http.StatusForbidden, map[string]string{"status": "error", "message": "forbidden"})
		metrics.ReqTotal.WithLabelValues("GET", "/api/img", "403").Inc()
		return
	}
	if img.Status != "ready" {
		writeJSON(w, http.StatusNotAcceptable, map[string]string{"status": "error", "message": "not ready"})
		metrics.ReqTotal.WithLabelValues("GET", "/api/img", "406").Inc()
		return
	}

	ext := h.cfg.S3.ExternalURL
	metrics.ReqTotal.WithLabelValues("GET", "/api/img", "200").Inc()
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"data": map[string]string{
			"original": ext + "/original/" + img.Name,
			"large":    ext + "/large/" + img.ID.String() + ".jpeg",
			"medium":   ext + "/medium/" + img.ID.String() + ".jpeg",
			"small":    ext + "/small/" + img.ID.String() + ".jpeg",
		},
	})
}
