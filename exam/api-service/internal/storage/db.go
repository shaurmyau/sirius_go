package storage

import (
	"context"
	"fmt"

	"api-service/internal/config"
	"api-service/internal/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DB struct {
	pool *pgxpool.Pool
}

func NewDB(cfg config.DBConfig) (*DB, error) {
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s", cfg.User, cfg.Pass, cfg.Host, cfg.Port, cfg.Name)
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		return nil, err
	}
	return &DB{pool: pool}, nil
}

func (d *DB) Close() { d.pool.Close() }

func (d *DB) CreateImage(ctx context.Context, img *model.Image) error {
	_, err := d.pool.Exec(ctx,
		`INSERT INTO public.image (id, user_id, name, mime_type, size, status)
         VALUES ($1, $2, $3, $4, $5, 'upload')`,
		img.ID, img.UserID, img.Name, img.MimeType, img.Size)
	return err
}

func (d *DB) GetImage(ctx context.Context, id uuid.UUID) (*model.Image, error) {
	row := d.pool.QueryRow(ctx,
		`SELECT id, user_id, name, mime_type, uploaded_at, size, size_large, size_medium, size_small, status
         FROM public.image WHERE id = $1`, id)
	img := &model.Image{}
	err := row.Scan(&img.ID, &img.UserID, &img.Name, &img.MimeType, &img.UploadedAt,
		&img.Size, &img.SizeLarge, &img.SizeMedium, &img.SizeSmall, &img.Status)
	if err != nil {
		return nil, err
	}
	return img, nil
}
