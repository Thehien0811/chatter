package app

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type objectStorage struct {
	client *minio.Client
	bucket string
}

func newObjectStorage(endpoint, accessKey, secretKey, bucket string, secure bool) (*objectStorage, error) {
	if strings.TrimSpace(endpoint) == "" || strings.TrimSpace(accessKey) == "" ||
		strings.TrimSpace(secretKey) == "" || strings.TrimSpace(bucket) == "" {
		return nil, fmt.Errorf("MinIO endpoint, access key, secret key, and bucket are required")
	}
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: secure,
	})
	if err != nil {
		return nil, fmt.Errorf("create MinIO client: %w", err)
	}
	return &objectStorage{client: client, bucket: bucket}, nil
}

func (s *objectStorage) put(ctx context.Context, key string, data []byte, contentType string) error {
	if err := s.ensureBucket(ctx); err != nil {
		return err
	}
	_, err := s.client.PutObject(ctx, s.bucket, key, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		return fmt.Errorf("store image in MinIO: %w", err)
	}
	return nil
}

func (s *objectStorage) remove(ctx context.Context, key string) error {
	if err := s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("remove image from MinIO: %w", err)
	}
	return nil
}

func (s *objectStorage) get(ctx context.Context, key string) (io.ReadCloser, minio.ObjectInfo, error) {
	object, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, minio.ObjectInfo{}, fmt.Errorf("open image from MinIO: %w", err)
	}
	info, err := object.Stat()
	if err != nil {
		_ = object.Close()
		return nil, minio.ObjectInfo{}, fmt.Errorf("read image metadata from MinIO: %w", err)
	}
	return object, info, nil
}

func (s *objectStorage) ensureBucket(ctx context.Context) error {
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return fmt.Errorf("check MinIO bucket: %w", err)
	}
	if exists {
		return nil
	}
	if err := s.client.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{}); err != nil {
		exists, checkErr := s.client.BucketExists(ctx, s.bucket)
		if checkErr == nil && exists {
			return nil
		}
		return fmt.Errorf("create MinIO bucket: %w", err)
	}
	return nil
}
