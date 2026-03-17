package service

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

type AWSSecretsManagerBillingSecretStoreConfig struct {
	Region          string
	Endpoint        string
	Prefix          string
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
}

type AWSSecretsManagerBillingSecretStore struct {
	client *secretsmanager.Client
	prefix string
}

func NewAWSSecretsManagerBillingSecretStore(ctx context.Context, cfg AWSSecretsManagerBillingSecretStoreConfig) (*AWSSecretsManagerBillingSecretStore, error) {
	cfg.Region = strings.TrimSpace(cfg.Region)
	if cfg.Region == "" {
		cfg.Region = "us-east-1"
	}
	cfg.Endpoint = strings.TrimSpace(cfg.Endpoint)
	if cfg.Endpoint != "" {
		if _, err := url.ParseRequestURI(cfg.Endpoint); err != nil {
			return nil, fmt.Errorf("invalid secrets manager endpoint: %w", err)
		}
	}
	prefix := strings.Trim(strings.TrimSpace(cfg.Prefix), "/")
	if prefix == "" {
		prefix = "alpha/billing-provider-connections"
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
				if service == secretsmanager.ServiceID {
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

	return &AWSSecretsManagerBillingSecretStore{
		client: secretsmanager.NewFromConfig(awsCfg),
		prefix: prefix,
	}, nil
}

func (s *AWSSecretsManagerBillingSecretStore) PutStripeSecret(ctx context.Context, connectionID, secret string) (string, error) {
	if s == nil || s.client == nil {
		return "", fmt.Errorf("aws secrets manager client is not initialized")
	}
	connectionID = strings.TrimSpace(connectionID)
	secret = strings.TrimSpace(secret)
	if connectionID == "" {
		return "", fmt.Errorf("%w: connection id is required", ErrValidation)
	}
	if secret == "" {
		return "", fmt.Errorf("%w: stripe secret is required", ErrValidation)
	}

	name := s.secretName(connectionID)
	out, err := s.client.CreateSecret(ctx, &secretsmanager.CreateSecretInput{
		Name:         aws.String(name),
		Description:  aws.String("Alpha billing provider connection secret"),
		SecretString: aws.String(secret),
	})
	if err != nil {
		return "", fmt.Errorf("create billing secret %q: %w", name, err)
	}
	if arn := strings.TrimSpace(aws.ToString(out.ARN)); arn != "" {
		return arn, nil
	}
	return name, nil
}

func (s *AWSSecretsManagerBillingSecretStore) GetStripeSecret(ctx context.Context, secretRef string) (string, error) {
	if s == nil || s.client == nil {
		return "", fmt.Errorf("aws secrets manager client is not initialized")
	}
	secretRef = strings.TrimSpace(secretRef)
	if secretRef == "" {
		return "", fmt.Errorf("%w: secret ref is required", ErrValidation)
	}
	out, err := s.client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{SecretId: aws.String(secretRef)})
	if err != nil {
		return "", fmt.Errorf("get billing secret %q: %w", secretRef, err)
	}
	secret := strings.TrimSpace(aws.ToString(out.SecretString))
	if secret == "" {
		return "", fmt.Errorf("billing secret %q is empty", secretRef)
	}
	return secret, nil
}

func (s *AWSSecretsManagerBillingSecretStore) RotateStripeSecret(ctx context.Context, secretRef, secret string) (string, error) {
	if s == nil || s.client == nil {
		return "", fmt.Errorf("aws secrets manager client is not initialized")
	}
	secretRef = strings.TrimSpace(secretRef)
	secret = strings.TrimSpace(secret)
	if secretRef == "" {
		return "", fmt.Errorf("%w: secret ref is required", ErrValidation)
	}
	if secret == "" {
		return "", fmt.Errorf("%w: stripe secret is required", ErrValidation)
	}
	_, err := s.client.PutSecretValue(ctx, &secretsmanager.PutSecretValueInput{
		SecretId:     aws.String(secretRef),
		SecretString: aws.String(secret),
	})
	if err != nil {
		return "", fmt.Errorf("rotate billing secret %q: %w", secretRef, err)
	}
	return secretRef, nil
}

func (s *AWSSecretsManagerBillingSecretStore) DeleteSecret(ctx context.Context, secretRef string) error {
	if s == nil || s.client == nil {
		return fmt.Errorf("aws secrets manager client is not initialized")
	}
	secretRef = strings.TrimSpace(secretRef)
	if secretRef == "" {
		return nil
	}
	_, err := s.client.DeleteSecret(ctx, &secretsmanager.DeleteSecretInput{
		SecretId:                   aws.String(secretRef),
		ForceDeleteWithoutRecovery: aws.Bool(true),
	})
	if err != nil {
		return fmt.Errorf("delete billing secret %q: %w", secretRef, err)
	}
	return nil
}

func (s *AWSSecretsManagerBillingSecretStore) secretName(connectionID string) string {
	return s.prefix + "/" + strings.TrimSpace(connectionID) + "/stripe"
}
