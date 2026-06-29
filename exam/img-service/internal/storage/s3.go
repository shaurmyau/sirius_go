package storage

import (
	"context"
	"fmt"
	"io"

	"img-service/internal/config"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type S3 struct{ client *minio.Client }

func NewS3(cfg config.S3Config) (*S3, error) {
	endpoint := fmt.Sprintf("%s:%s", cfg.Host, cfg.Port)
	c, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.User, cfg.Pass, ""),
		Secure: false,
	})
	if err != nil {
		return nil, err
	}
	return &S3{client: c}, nil
}

func (s *S3) Get(ctx context.Context, bucket, object string) (io.ReadCloser, error) {
	obj, err := s.client.GetObject(ctx, bucket, object, minio.GetObjectOptions{})
	return obj, err
}

func (s *S3) Put(ctx context.Context, bucket, object string, r io.Reader, size int64, contentType string) error {
	_, err := s.client.PutObject(ctx, bucket, object, r, size, minio.PutObjectOptions{ContentType: contentType})
	return err
}

func (s *S3) Copy(ctx context.Context, srcBucket, srcObject, dstBucket, dstObject string) error {
	src := minio.CopySrcOptions{Bucket: srcBucket, Object: srcObject}
	dst := minio.CopyDestOptions{Bucket: dstBucket, Object: dstObject}
	_, err := s.client.CopyObject(ctx, dst, src)
	if err != nil {
		return err
	}
	return s.client.RemoveObject(ctx, srcBucket, srcObject, minio.RemoveObjectOptions{})
}
