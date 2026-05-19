// Package s3 wires the AWS SDK v2 client against Cloudflare R2. The
// adapter that *uses* it lives in `adapter/storage/r2/`; this file
// contains only the boring SDK plumbing.
package s3

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// R2Settings groups the credentials and identifiers needed to
// construct the SDK client.
type R2Settings struct {
	AccountID       string
	AccessKeyID     string
	SecretAccessKey string
}

// NewR2Client constructs an SDK client pinned to the R2 endpoint.
// `region: "auto"` is required by the SDK and ignored by R2.
func NewR2Client(ctx context.Context, s R2Settings) (*s3.Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(s.AccessKeyID, s.SecretAccessKey, ""),
		),
		config.WithRegion("auto"),
		// Pinning checksum behaviour: aws-sdk-go-v2 ≥ 1.73 changed the
		// default to "WhenSupported", which R2 rejects. WhenRequired
		// preserves compatibility without downgrading the SDK.
		config.WithRequestChecksumCalculation(aws.RequestChecksumCalculationWhenRequired),
	)
	if err != nil {
		return nil, fmt.Errorf("s3: load config: %w", err)
	}
	endpoint := fmt.Sprintf("https://%s.r2.cloudflarestorage.com", s.AccountID)
	return s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		// R2 requires path-style addressing.
		o.UsePathStyle = true
	}), nil
}
