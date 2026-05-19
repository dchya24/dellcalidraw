package storage

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type StorageClient struct {
	client   *s3.Client
	presign  *s3.PresignClient
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
	scheme := "http"
	if cfg.UseSSL {
		scheme = "https"
	}
	endpointURL := fmt.Sprintf("%s://%s", scheme, cfg.Endpoint)

	awsCfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(cfg.Region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AccessKey, cfg.SecretKey, "",
		)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpointURL)
		o.UsePathStyle = true
	})

	presignClient := s3.NewPresignClient(client)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Check if bucket exists
	_, err = client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(cfg.Bucket),
	})
	if err != nil {
		// Bucket doesn't exist, create it
		_, createErr := client.CreateBucket(ctx, &s3.CreateBucketInput{
			Bucket: aws.String(cfg.Bucket),
			CreateBucketConfiguration: &types.CreateBucketConfiguration{
				LocationConstraint: types.BucketLocationConstraint(cfg.Region),
			},
		})
		if createErr != nil {
			return nil, fmt.Errorf("failed to create bucket: %w", createErr)
		}
		slog.Info("Created storage bucket", "bucket", cfg.Bucket)
	}

	slog.Info("Storage client connected", "endpoint", cfg.Endpoint, "bucket", cfg.Bucket)
	return &StorageClient{
		client:   client,
		presign:  presignClient,
		bucket:   cfg.Bucket,
		endpoint: cfg.Endpoint,
		public:   cfg.Public,
	}, nil
}

func (s *StorageClient) Upload(ctx context.Context, key string, reader io.Reader, size int64, contentType string) error {
	input := &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(key),
		Body:          reader,
		ContentLength: aws.Int64(size),
		ContentType:   aws.String(contentType),
		ACL:           types.ObjectCannedACLPublicRead,
	}

	_, err := s.client.PutObject(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to upload file: %w", err)
	}

	slog.Info("File uploaded", "key", key, "size", size)
	return nil
}

func (s *StorageClient) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	output, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to download file: %w", err)
	}
	return output.Body, nil
}

func (s *StorageClient) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
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
	output, err := s.presign.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(expiry))
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}
	return output.URL, nil
}

type ObjectInfo struct {
	Key          string
	Size         int64
	ContentType  string
	ETag         string
	LastModified time.Time
}

func (s *StorageClient) StatObject(ctx context.Context, key string) (ObjectInfo, error) {
	output, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return ObjectInfo{}, fmt.Errorf("failed to stat object: %w", err)
	}

	contentType := ""
	if output.ContentType != nil {
		contentType = *output.ContentType
	}
	etag := ""
	if output.ETag != nil {
		etag = *output.ETag
	}
	var lastModified time.Time
	if output.LastModified != nil {
		lastModified = *output.LastModified
	}

	return ObjectInfo{
		Key:          key,
		Size:         aws.ToInt64(output.ContentLength),
		ContentType:  contentType,
		ETag:         etag,
		LastModified: lastModified,
	}, nil
}

func (s *StorageClient) Close() {
	slog.Info("Storage client closed")
}

// httpClient is kept for potential custom transport needs
var _ http.RoundTripper = http.DefaultTransport
