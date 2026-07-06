package storage

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
)

type Storage interface {
	Upload(ctx context.Context, key string, data []byte, contentType string) error
	Delete(ctx context.Context, key string) error
	GetURL(key string) string
	GenerateKey(workspaceID, filename string, t time.Time) string
	ExtractKey(url string) string
}

type R2Config struct {
	AccountID  string
	AccessKey  string
	SecretKey  string
	BucketName string
	PublicURL  string
}

type R2Storage struct {
	client     *s3.Client
	bucketName string
	publicURL  string
}

func NewR2Storage(cfg R2Config) (*R2Storage, error) {
	if cfg.AccountID == "" || cfg.AccessKey == "" || cfg.SecretKey == "" || cfg.BucketName == "" {
		return nil, fmt.Errorf("R2 configuration is incomplete")
	}

	r2Credentials := credentials.NewStaticCredentialsProvider(
		cfg.AccessKey,
		cfg.SecretKey,
		"",
	)

	customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		return aws.Endpoint{
			URL:               fmt.Sprintf("https://%s.r2.cloudflarestorage.com", cfg.AccountID),
			SigningRegion:     "auto",
			HostnameImmutable: true,
		}, nil
	})

	awsCfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithCredentialsProvider(r2Credentials),
		config.WithEndpointResolverWithOptions(customResolver),
		config.WithRegion("auto"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = true
	})

	return &R2Storage{
		client:     client,
		bucketName: cfg.BucketName,
		publicURL:  cfg.PublicURL,
	}, nil
}

func (r *R2Storage) Upload(ctx context.Context, key string, data []byte, contentType string) error {
	_, err := r.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(r.bucketName),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
	})
	return err
}

func (r *R2Storage) Delete(ctx context.Context, key string) error {
	_, err := r.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(r.bucketName),
		Key:    aws.String(key),
	})
	return err
}

func (r *R2Storage) GetURL(key string) string {
	if r.publicURL != "" {
		return r.publicURL + "/" + key
	}
	return key
}

func (r *R2Storage) GenerateKey(workspaceID, filename string, t time.Time) string {
	cleanFilename := strings.ReplaceAll(filename, " ", "_")
	uniqueID := uuid.New().String()
	return fmt.Sprintf("workspace-%s/media/%d/%02d/%s_%s", workspaceID, t.Year(), int(t.Month()), uniqueID, cleanFilename)
}

func (r *R2Storage) ExtractKey(url string) string {
	if r.publicURL != "" && strings.HasPrefix(url, r.publicURL) {
		return strings.TrimPrefix(url, r.publicURL+"/")
	}
	if strings.HasPrefix(url, "/uploads/") {
		parts := strings.Split(url, "/")
		if len(parts) >= 3 {
			return strings.Join(parts[2:], "/")
		}
	}
	return url
}

type LocalStorage struct {
	basePath string
}

func NewLocalStorage(basePath string) *LocalStorage {
	return &LocalStorage{basePath: basePath}
}

func (l *LocalStorage) Upload(ctx context.Context, key string, data []byte, contentType string) error {
	filePath := l.basePath + "/" + key

	dir := l.basePath + "/" + key
	lastSlash := -1
	for i := len(dir) - 1; i >= 0; i-- {
		if dir[i] == '/' {
			lastSlash = i
			break
		}
	}
	if lastSlash > 0 {
		dir = dir[:lastSlash]
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	return os.WriteFile(filePath, data, 0644)
}

func (l *LocalStorage) Delete(ctx context.Context, key string) error {
	filePath := l.basePath + "/" + key
	return os.Remove(filePath)
}

func (l *LocalStorage) GetURL(key string) string {
	return "/uploads/" + key
}

func (l *LocalStorage) GenerateKey(workspaceID, filename string, t time.Time) string {
	cleanFilename := strings.ReplaceAll(filename, " ", "_")
	uniqueID := uuid.New().String()
	return fmt.Sprintf("%s_%s", uniqueID, cleanFilename)
}

func (l *LocalStorage) ExtractKey(url string) string {
	if strings.HasPrefix(url, "/uploads/") {
		return strings.TrimPrefix(url, "/uploads/")
	}
	return url
}
