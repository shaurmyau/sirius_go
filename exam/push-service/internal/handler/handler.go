package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"push-service/internal/storage"

	"github.com/google/uuid"
)

type Handler struct {
	db     *storage.DB
	logger *slog.Logger
}

func New(db *storage.DB, logger *slog.Logger) *Handler {
	return &Handler{db: db, logger: logger}
}

func (h *Handler) Notify(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Object   string `json:"object"`
		ObjectID string `json:"object_id"`
		UserID   string `json:"user_id"`
		Status   string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Warn("bad notify payload", "err", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	id, err := uuid.Parse(req.ObjectID)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	info, err := h.db.GetImageInfo(r.Context(), id)
	if err != nil {
		h.logger.Error("db get image info failed", "err", err, "id", id)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	h.logger.Info("notify",
		"log", "notify",
		"user_id", info.UserID.String(),
		"object", req.Object,
		"object_id", req.ObjectID,
		"filename", info.Name,
		"size", info.Size,
		"status", req.Status,
	)

	w.WriteHeader(http.StatusOK)
}
