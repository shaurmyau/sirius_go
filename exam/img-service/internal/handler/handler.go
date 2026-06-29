package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"img-service/internal/queue"

	"github.com/google/uuid"
)

type Handler struct {
	q      *queue.Queue
	logger *slog.Logger
}

func New(q *queue.Queue, logger *slog.Logger) *Handler {
	return &Handler{q: q, logger: logger}
}

func (h *Handler) Process(w http.ResponseWriter, r *http.Request) {
	// Minio webhook payload – extract object name (= image UUID)
	var payload struct {
		Records []struct {
			S3 struct {
				Object struct {
					Key string `json:"key"`
				} `json:"object"`
			} `json:"s3"`
		} `json:"Records"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || len(payload.Records) == 0 {
		h.logger.Warn("bad webhook payload", "err", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	key := payload.Records[0].S3.Object.Key
	id, err := uuid.Parse(key)
	if err != nil {
		h.logger.Warn("invalid object key", "key", key)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	h.q.Enqueue(id)
	w.WriteHeader(http.StatusAccepted)
}
