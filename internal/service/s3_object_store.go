package service

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type ObjectStore interface {
	EnsureBucket(ctx context.Context) error
	PutObject(ctx context.Context, key string, body []byte, contentType string) error
	PresignGetObject(ctx context.Context, key string, expires time.Duration) (string, error)
}

type S3Config struct {
	Region          string
	Bucket          string
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
	ForcePathStyle  bool
}

type S3ObjectStore struct {
	bucket    string
	client    *s3.Client
	presigner *s3.PresignClient
}

func NewS3ObjectStore(ctx context.Context, cfg S3Config) (*S3ObjectStore, error) {
	cfg.Region = strings.TrimSpace(cfg.Region)
	if cfg.Region == "" {
		cfg.Region = "us-east-1"
	}
	cfg.Bucket = strings.TrimSpace(cfg.Bucket)
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("s3 bucket is required")
	}
	cfg.Endpoint = strings.TrimSpace(cfg.Endpoint)
	if cfg.Endpoint != "" {
		if _, err := url.ParseRequestURI(cfg.Endpoint); err != nil {
			return nil, fmt.Errorf("invalid s3 endpoint: %w", err)
		}
	}

	loadOpts := []func(*config.LoadOptions) error{
		config.WithRegion(cfg.Region),
	}
	if cfg.AccessKeyID != "" || cfg.SecretAccessKey != "" {
		loadOpts = append(loadOpts, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, cfg.SessionToken),
		))
	}
	if cfg.Endpoint != "" {
		loadOpts = append(loadOpts, config.WithEndpointResolverWithOptions(
			aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
				if service == s3.ServiceID {
					return aws.Endpoint{
						URL:               cfg.Endpoint,
						HostnameImmutable: true,
						SigningRegion:     cfg.Region,
					}, nil
				}
				return aws.Endpoint{}, &aws.EndpointNotFoundError{}
			}),
		))
	}

	awsCfg, err := config.LoadDefaultConfig(ctx, loadOpts...)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = cfg.ForcePathStyle
	})
	return &S3ObjectStore{
		bucket:    cfg.Bucket,
		client:    client,
		presigner: s3.NewPresignClient(client),
	}, nil
}

func (s *S3ObjectStore) EnsureBucket(ctx context.Context) error {
	if s == nil || s.client == nil {
		return fmt.Errorf("s3 client is not initialized")
	}
	_, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(s.bucket),
	})
	if err == nil {
		return nil
	}

	_, createErr := s.client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(s.bucket),
	})
	if createErr != nil {
		return fmt.Errorf("ensure bucket %q: %w", s.bucket, createErr)
	}
	return nil
}

func (s *S3ObjectStore) PutObject(ctx context.Context, key string, body []byte, contentType string) error {
	if s == nil || s.client == nil {
		return fmt.Errorf("s3 client is not initialized")
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("object key is required")
	}
	contentType = strings.TrimSpace(contentType)
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(body),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return fmt.Errorf("put object %q: %w", key, err)
	}
	return nil
}

func (s *S3ObjectStore) PresignGetObject(ctx context.Context, key string, expires time.Duration) (string, error) {
	if s == nil || s.presigner == nil {
		return "", fmt.Errorf("s3 presigner is not initialized")
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return "", fmt.Errorf("object key is required")
	}
	if expires <= 0 {
		expires = 5 * time.Minute
	}
	req, err := s.presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = expires
	})
	if err != nil {
		return "", fmt.Errorf("presign object %q: %w", key, err)
	}
	return req.URL, nil
}
