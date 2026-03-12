// Package s3 provides a thin wrapper around the AWS SDK v2 S3 client.
// It supports LocalStack for local development via S3_ENDPOINT env var.
package s3

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/rs/zerolog"

	pkgconfig "github.com/bhata/AutoDreamApplier/pkg/config"
)

// Client wraps an AWS S3 client with helpers for AutoDreamApplier.
type Client struct {
	s3c     *s3.Client
	presign *s3.PresignClient
	log     zerolog.Logger
}

// New creates an S3 Client from the supplied configuration.
// When cfg.S3.Endpoint is non-empty (e.g. LocalStack), path-style access
// and a custom endpoint are configured automatically.
func New(ctx context.Context, s3cfg pkgconfig.S3Config, awscfg pkgconfig.AWSConfig, log zerolog.Logger) (*Client, error) {
	opts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(awscfg.Region),
	}

	// Use static credentials only when both key ID and secret are provided
	// (IAM roles are used in production; static creds are for local/CI).
	if awscfg.AccessKeyID != "" && awscfg.SecretAccessKey != "" {
		opts = append(opts, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(
				awscfg.AccessKeyID,
				awscfg.SecretAccessKey,
				"",
			),
		))
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	s3opts := []func(*s3.Options){}

	// LocalStack / custom endpoint support
	if s3cfg.Endpoint != "" {
		endpoint := s3cfg.Endpoint
		s3opts = append(s3opts, func(o *s3.Options) {
			o.BaseEndpoint = &endpoint
			o.UsePathStyle = true
		})
	}

	s3c := s3.NewFromConfig(cfg, s3opts...)
	log.Info().
		Str("region", awscfg.Region).
		Str("endpoint", s3cfg.Endpoint).
		Msg("S3 client initialized")

	return &Client{
		s3c:     s3c,
		presign: s3.NewPresignClient(s3c),
		log:     log,
	}, nil
}

// ── Write helpers ─────────────────────────────────────────────────────────────

// PutText uploads plain-text content to the given bucket/key.
func (c *Client) PutText(ctx context.Context, bucket, key, text string) error {
	return c.PutBytes(ctx, bucket, key, []byte(text), "text/plain; charset=utf-8")
}

// PutBytes uploads raw bytes to the given bucket/key with the specified content type.
func (c *Client) PutBytes(ctx context.Context, bucket, key string, data []byte, contentType string) error {
	contentLen := int64(len(data))
	_, err := c.s3c.PutObject(ctx, &s3.PutObjectInput{
		Bucket:               &bucket,
		Key:                  &key,
		Body:                 bytes.NewReader(data),
		ContentLength:        &contentLen,
		ContentType:          &contentType,
		ServerSideEncryption: types.ServerSideEncryptionAes256,
	})
	if err != nil {
		return fmt.Errorf("s3 put %s/%s: %w", bucket, key, err)
	}
	c.log.Debug().Str("bucket", bucket).Str("key", key).Int("bytes", len(data)).Msg("S3 put object")
	return nil
}

// ── Read helpers ──────────────────────────────────────────────────────────────

// GetText downloads an object and returns its content as a string.
func (c *Client) GetText(ctx context.Context, bucket, key string) (string, error) {
	out, err := c.s3c.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	})
	if err != nil {
		return "", fmt.Errorf("s3 get %s/%s: %w", bucket, key, err)
	}
	defer out.Body.Close()

	data, err := io.ReadAll(out.Body)
	if err != nil {
		return "", fmt.Errorf("s3 read body %s/%s: %w", bucket, key, err)
	}
	return string(data), nil
}

// ── Presigned URLs ────────────────────────────────────────────────────────────

// PresignedURL returns a time-limited presigned GET URL for the object.
func (c *Client) PresignedURL(ctx context.Context, bucket, key string, expiry time.Duration) (string, error) {
	req, err := c.presign.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	}, s3.WithPresignExpires(expiry))
	if err != nil {
		return "", fmt.Errorf("s3 presign %s/%s: %w", bucket, key, err)
	}
	return req.URL, nil
}

// ── Key helpers ───────────────────────────────────────────────────────────────

// ResumeKey returns the S3 key for a tailored resume text file.
func ResumeKey(appID string) string {
	return "tailored-resumes/" + appID + ".txt"
}

// CoverLetterKey returns the S3 key for a cover letter text file.
func CoverLetterKey(appID string) string {
	return "cover-letters/" + appID + ".txt"
}

// ScreenshotKey returns the S3 key for a submission screenshot.
func ScreenshotKey(appID string) string {
	return "screenshots/" + appID + ".png"
}
