package storage

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"api-service/internal/config"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type S3 struct {
	client      *minio.Client
	externalURL string
}

func NewS3(cfg config.S3Config) (*S3, error) {
	endpoint := fmt.Sprintf("%s:%s", cfg.Host, cfg.Port)
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.User, cfg.Pass, ""),
		Secure: false,
	})
	if err != nil {
		return nil, err
	}
	return &S3{client: client, externalURL: cfg.ExternalURL}, nil
}

func (s *S3) PresignedPutURL(ctx context.Context, objectName string) (*url.URL, error) {
	u, err := s.client.PresignedPutObject(ctx, "upload", objectName, 15*time.Minute)
	if err != nil {
		return nil, err
	}
	if s.externalURL != "" {
		ext, parseErr := url.Parse(s.externalURL)
		if parseErr == nil && ext.Host != "" {
			u.Scheme = ext.Scheme
			u.Host = ext.Host
		}
	}
	return u, nil
}
