package storage

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type S3Config struct {
	Endpoint string
	Region   string
	Bucket   string
}

func NewS3StorageFromEnv(ctx context.Context) (*S3Storage, error) {
	cfg := S3Config{
		Endpoint: os.Getenv("S3_ENDPOINT"),
		Region:   os.Getenv("S3_REGION"),
		Bucket:   os.Getenv("S3_BUCKET"),
	}

	if cfg.Region == "" {
		return nil, fmt.Errorf("S3_REGION is not set")
	}

	if cfg.Bucket == "" {
		return nil, fmt.Errorf("S3_BUCKET is not set")
	}

	awsCfg, err := config.LoadDefaultConfig(
		ctx,
		config.WithRegion(cfg.Region),
	)
	if err != nil {
		return nil, err
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = &cfg.Endpoint
			o.UsePathStyle = true
		}
	})

	return NewS3Storage(client, cfg.Bucket), nil
}
