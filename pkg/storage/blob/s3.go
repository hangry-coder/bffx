package blob

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// S3Provider issues presigned URLs using the MinIO client (S3-compatible).
// Configure with BFFX_S3_ENDPOINT, BFFX_S3_ACCESS_KEY, BFFX_S3_SECRET_KEY, BFFX_S3_BUCKET,
// optional BFFX_S3_REGION, BFFX_S3_USE_SSL (true/false when endpoint has no scheme).
type S3Provider struct {
	client *minio.Client
	bucket string
}

// NewS3ProviderFromEnv builds an S3Provider from environment variables.
func NewS3ProviderFromEnv() (*S3Provider, error) {
	endpoint := strings.TrimSpace(os.Getenv("BFFX_S3_ENDPOINT"))
	ak := strings.TrimSpace(os.Getenv("BFFX_S3_ACCESS_KEY"))
	sk := strings.TrimSpace(os.Getenv("BFFX_S3_SECRET_KEY"))
	bucket := strings.TrimSpace(os.Getenv("BFFX_S3_BUCKET"))
	if endpoint == "" || ak == "" || sk == "" || bucket == "" {
		return nil, fmt.Errorf("%w", ErrNotConfigured)
	}

	secure := strings.EqualFold(strings.TrimSpace(os.Getenv("BFFX_S3_USE_SSL")), "true")
	if strings.HasPrefix(endpoint, "https://") {
		secure = true
		endpoint = strings.TrimPrefix(endpoint, "https://")
	} else if strings.HasPrefix(endpoint, "http://") {
		secure = false
		endpoint = strings.TrimPrefix(endpoint, "http://")
	}

	region := strings.TrimSpace(os.Getenv("BFFX_S3_REGION"))

	cli, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(ak, sk, ""),
		Secure: secure,
		Region: region,
	})
	if err != nil {
		return nil, fmt.Errorf("blob: minio client: %w", err)
	}
	return &S3Provider{client: cli, bucket: bucket}, nil
}

func (s *S3Provider) PresignPut(ctx context.Context, _, objectKey string, ttl time.Duration) (PresignResult, error) {
	if err := ValidateObjectKey(objectKey); err != nil {
		return PresignResult{}, err
	}
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	u, err := s.client.PresignedPutObject(ctx, s.bucket, objectKey, ttl)
	if err != nil {
		return PresignResult{}, fmt.Errorf("blob: presign put: %w", err)
	}
	exp := time.Now().UTC().Add(ttl)
	return PresignResult{
		URL:       u.String(),
		Method:    http.MethodPut,
		ExpiresAt: exp,
	}, nil
}

func (s *S3Provider) PresignGet(ctx context.Context, _, objectKey string, ttl time.Duration) (PresignResult, error) {
	if err := ValidateObjectKey(objectKey); err != nil {
		return PresignResult{}, err
	}
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	u, err := s.client.PresignedGetObject(ctx, s.bucket, objectKey, ttl, url.Values{})
	if err != nil {
		return PresignResult{}, fmt.Errorf("blob: presign get: %w", err)
	}
	exp := time.Now().UTC().Add(ttl)
	return PresignResult{
		URL:       u.String(),
		Method:    http.MethodGet,
		ExpiresAt: exp,
	}, nil
}

// PublicURL returns a 24h presigned GET URL for S3 storage.
func (s *S3Provider) PublicURL(_, objectKey string) string {
	res, err := s.PresignGet(context.Background(), "", objectKey, 24*time.Hour)
	if err != nil {
		return ""
	}
	return res.URL
}

// Delete removes the object from S3.
func (s *S3Provider) Delete(ctx context.Context, objectKey string) error {
	err := s.client.RemoveObject(ctx, s.bucket, objectKey, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("blob: delete s3: %w", err)
	}
	return nil
}

// Type returns "s3".
func (s *S3Provider) Type() string {
	return "s3"
}

// Ping checks if the S3 bucket is accessible.
func (s *S3Provider) Ping(ctx context.Context) error {
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return fmt.Errorf("blob: ping s3: %w", err)
	}
	if !exists {
		return fmt.Errorf("blob: ping s3: bucket %q not found", s.bucket)
	}
	return nil
}
