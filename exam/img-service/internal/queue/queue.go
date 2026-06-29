// img-service/internal/queue/queue.go
package queue

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/gif"
	_ "image/png"
	"log/slog"
	"net/http"

	"img-service/internal/config"
	"img-service/internal/storage"

	"github.com/disintegration/imaging"
	"github.com/google/uuid"
)

type Queue struct {
	ch     chan uuid.UUID
	db     *storage.DB
	s3     *storage.S3
	cfg    *config.Config
	logger *slog.Logger
}

func New(db *storage.DB, s3 *storage.S3, cfg *config.Config, logger *slog.Logger) *Queue {
	return &Queue{ch: make(chan uuid.UUID, 64), db: db, s3: s3, cfg: cfg, logger: logger}
}

func (q *Queue) Enqueue(id uuid.UUID) { q.ch <- id }

func (q *Queue) Run() {
	for id := range q.ch {
		q.process(id)
	}
}

func (q *Queue) process(id uuid.UUID) {
	ctx := context.Background()
	log := q.logger.With("id", id)
	log.Info("processing started")

	fail := func(err error) {
		log.Error("processing failed", "err", err)
		q.db.SetStatus(ctx, id, "error")
		q.pushNotify(ctx, id, "error")
	}

	q.db.SetStatus(ctx, id, "process")

	rc, err := q.s3.Get(ctx, "upload", id.String())
	if err != nil {
		fail(fmt.Errorf("s3 get: %w", err))
		return
	}
	defer rc.Close()

	src, _, err := image.Decode(rc)
	if err != nil {
		fail(fmt.Errorf("decode: %w", err))
		return
	}

	sizes := []struct {
		bucket string
		w, h   int
	}{
		{"large", 1024, 1024},
		{"medium", 512, 512},
		{"small", 128, 128},
	}

	sizeBytes := map[string]int64{}
	for _, s := range sizes {
		resized := imaging.Fit(src, s.w, s.h, imaging.Lanczos)
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, resized, &jpeg.Options{Quality: 85}); err != nil {
			fail(fmt.Errorf("encode %s: %w", s.bucket, err))
			return
		}
		data := buf.Bytes()
		sizeBytes[s.bucket] = int64(len(data))
		if err := q.s3.Put(ctx, s.bucket, id.String()+".jpeg", bytes.NewReader(data), int64(len(data)), "image/jpeg"); err != nil {
			fail(fmt.Errorf("put %s: %w", s.bucket, err))
			return
		}
		log.Info("preview saved", "bucket", s.bucket)
	}

	// Получаем оригинальное имя файла из БД
	originalName, err := q.db.GetFileName(ctx, id)
	if err != nil {
		fail(fmt.Errorf("get original name: %w", err))
		return
	}

	// Переносим в original с оригинальным именем
	if err := q.s3.Copy(ctx, "upload", id.String(), "original", originalName); err != nil {
		fail(fmt.Errorf("copy original: %w", err))
		return
	}

	if err := q.db.UpdateSizes(ctx, id, sizeBytes["large"], sizeBytes["medium"], sizeBytes["small"]); err != nil {
		fail(fmt.Errorf("db update: %w", err))
		return
	}

	log.Info("processing done")
	q.pushNotify(ctx, id, "ready")
}

func (q *Queue) pushNotify(ctx context.Context, id uuid.UUID, status string) {
	userID, err := q.db.GetUserID(ctx, id)
	if err != nil {
		q.logger.Warn("get user_id failed", "err", err)
	}

	body, _ := json.Marshal(map[string]string{
		"object":    "image",
		"object_id": id.String(),
		"user_id":   userID.String(),
		"status":    status,
	})
	resp, err := http.Post(q.cfg.PushEndpoint, "application/json", bytes.NewReader(body))
	if err != nil {
		q.logger.Warn("push notify failed", "err", err)
		return
	}
	resp.Body.Close()
}