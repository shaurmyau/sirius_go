package storage

import (
	"context"
	"fmt"

	"img-service/internal/config"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

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

func (d *DB) SetStatus(ctx context.Context, id uuid.UUID, status string) error {
	_, err := d.pool.Exec(ctx, `UPDATE public.image SET status=$1 WHERE id=$2`, status, id)
	return err
}

func (d *DB) UpdateSizes(ctx context.Context, id uuid.UUID, large, medium, small int64) error {
	_, err := d.pool.Exec(ctx,
		`UPDATE public.image SET size_large=$1, size_medium=$2, size_small=$3, status='ready' WHERE id=$4`,
		large, medium, small, id)
	return err
}

func (d *DB) GetUserID(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
	var userID uuid.UUID
	err := d.pool.QueryRow(ctx, `SELECT user_id FROM public.image WHERE id=$1`, id).Scan(&userID)
	return userID, err
}

// GetFileName возвращает оригинальное имя файла по ID
func (d *DB) GetFileName(ctx context.Context, id uuid.UUID) (string, error) {
	var name string
	err := d.pool.QueryRow(ctx, `SELECT name FROM public.image WHERE id=$1`, id).Scan(&name)
	return name, err
}