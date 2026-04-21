package storage

import (
	"context"
	"fmt"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"io"
	"log/slog"
	"time"
)

type StorageClient struct {
	client   *minio.Client
	bucket   string
	endpoint string
	public   bool
}
type StorageConfig struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
	Region    string
	UseSSL    bool
	Public    bool
}

func NewStorageClient(cfg StorageConfig) (*StorageClient, error) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Region: cfg.Region,
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create storage client: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	exists, err := client.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		return nil, fmt.Errorf("failed to check bucket: %w", err)
	}
	if !exists {
		if err := client.MakeBucket(ctx, cfg.Bucket, minio.MakeBucketOptions{Region: cfg.Region}); err != nil {
			return nil, fmt.Errorf("failed to create bucket: %w", err)
		}
		slog.Info("Created storage bucket", "bucket", cfg.Bucket)
	}
	slog.Info("Storage client connected", "endpoint", cfg.Endpoint, "bucket", cfg.Bucket)
	return &StorageClient{
		client:   client,
		bucket:   cfg.Bucket,
		endpoint: cfg.Endpoint,
		public:   cfg.Public,
	}, nil
}
func (s *StorageClient) Upload(ctx context.Context, key string, reader io.Reader, size int64, contentType string) error {
	opts := minio.PutObjectOptions{
		ContentType: contentType,
		UserMetadata: map[string]string{
			"x-amz-acl": "public-read",
		},
	}
	info, err := s.client.PutObject(ctx, s.bucket, key, reader, size, opts)
	if err != nil {
		return fmt.Errorf("failed to upload file: %w", err)
	}
	slog.Info("File uploaded", "key", key, "size", info.Size, "etag", info.ETag)
	return nil
}
func (s *StorageClient) Download(ctx context.Context, key string) (*minio.Object, error) {
	obj, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to download file: %w", err)
	}
	return obj, nil
}
func (s *StorageClient) Delete(ctx context.Context, key string) error {
	err := s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}
	slog.Info("File deleted", "key", key)
	return nil
}
func (s *StorageClient) GetURL(key string) string {
	if s.public {
		return fmt.Sprintf("http://%s/%s/%s", s.endpoint, s.bucket, key)
	}
	return fmt.Sprintf("http://%s/%s/%s", s.endpoint, s.bucket, key)
}
func (s *StorageClient) PresignedGetURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	u, err := s.client.PresignedGetObject(ctx, s.bucket, key, expiry, nil)
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}
	return u.String(), nil
}
func (s *StorageClient) StatObject(ctx context.Context, key string) (minio.ObjectInfo, error) {
	info, err := s.client.StatObject(ctx, s.bucket, key, minio.StatObjectOptions{})
	if err != nil {
		return minio.ObjectInfo{}, fmt.Errorf("failed to stat object: %w", err)
	}
	return info, nil
}
func (s *StorageClient) Close() {
	slog.Info("Storage client closed")
}
