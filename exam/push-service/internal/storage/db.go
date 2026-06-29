package storage

import (
	"context"
	"fmt"

	"push-service/internal/config"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ImageInfo struct {
	Name   string
	Size   int64
	UserID uuid.UUID
}

type DB struct{ pool *pgxpool.Pool }

func NewDB(cfg config.DBConfig) (*DB, error) {
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s", cfg.User, cfg.Pass, cfg.Host, cfg.Port, cfg.Name)
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		return nil, err
	}
	return &DB{pool: pool}, nil
}

func (d *DB) Close() { d.pool.Close() }

func (d *DB) GetImageInfo(ctx context.Context, id uuid.UUID) (*ImageInfo, error) {
	info := &ImageInfo{}
	err := d.pool.QueryRow(ctx, `SELECT name, size, user_id FROM public.image WHERE id=$1`, id).
		Scan(&info.Name, &info.Size, &info.UserID)
	return info, err
}
