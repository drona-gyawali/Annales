package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"

	"github.com/google/uuid"

	"github.com/drona-gyawali/logStore/internal/log"
	"github.com/drona-gyawali/logStore/internal/manifest"
)

type S3Storage struct {
	TestHookBeforeManifestCommit func() error

	client *s3.Client
	bucket string
}

func NewS3Storage(client *s3.Client, bucket string) *S3Storage {
	return &S3Storage{
		client: client,
		bucket: bucket,
	}
}

func (s *S3Storage) Get(
	ctx context.Context,
	key string,
) ([]byte, string, error) {

	output, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = output.Body.Close() }()

	data, err := io.ReadAll(output.Body)
	if err != nil {
		return nil, "", err
	}

	if output.ETag == nil {
		return nil, "", fmt.Errorf("object %q returned no ETag", key)
	}

	return data, *output.ETag, nil
}

func (s *S3Storage) Put(
	ctx context.Context,
	key string,
	data []byte,
) (string, error) {

	output, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(data),
	})
	if err != nil {
		return "", err
	}

	if output.ETag == nil {
		return "", fmt.Errorf("object %q returned no ETag", key)
	}

	return *output.ETag, nil
}

func (s *S3Storage) PutIfMatch(
	ctx context.Context,
	key string,
	data []byte,
	etag string,
) (string, error) {

	output, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:  aws.String(s.bucket),
		Key:     aws.String(key),
		Body:    bytes.NewReader(data),
		IfMatch: aws.String(etag),
	})
	if err != nil {
		return "", err
	}

	if output.ETag == nil {
		return "", fmt.Errorf("object %q returned no ETag", key)
	}

	return *output.ETag, nil
}

func (s *S3Storage) PutIfNoneMatch(
	ctx context.Context,
	key string,
	data []byte,
) (string, error) {

	output, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		IfNoneMatch: aws.String("*"),
	})
	if err != nil {
		return "", err
	}

	if output.ETag == nil {
		return "", fmt.Errorf("object %q returned no ETag", key)
	}

	return *output.ETag, nil
}

func isPreconditionFailed(err error) bool {
	var apiErr smithy.APIError

	if !errors.As(err, &apiErr) {
		return false
	}

	return apiErr.ErrorCode() == "PreconditionFailed"
}

func (s *S3Storage) Delete(
	ctx context.Context,
	key string,
) error {

	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})

	return err
}

func (s *S3Storage) Append(
	ctx context.Context,
	operations []log.LogEntry,
) error {

	for {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("append cancelled: %w", err)
		}

		data, etag, err := s.Get(ctx, "index.json")
		if err != nil {
			return fmt.Errorf("get manifest: %w", err)
		}

		var current manifest.IndexManifest

		if err := json.Unmarshal(data, &current); err != nil {
			return fmt.Errorf("decode manifest: %w", err)
		}

		nextSeq := current.CurrentSeq + 1

		batch := log.LogBatch{
			SequenceID: nextSeq,
			PrevLogKey: current.CurrentLogKey,
			Operations: operations,
		}

		logData, err := json.Marshal(batch)
		if err != nil {
			return fmt.Errorf("encode log batch: %w", err)
		}

		logKey := fmt.Sprintf(
			"logs/%d-%s.json",
			nextSeq,
			uuid.NewString(),
		)

		if _, err := s.Put(ctx, logKey, logData); err != nil {
			return fmt.Errorf("upload log: %w", err)
		}

		nextManifest := manifest.IndexManifest{
			CurrentSeq:         nextSeq,
			CurrentLogKey:      logKey,
			CurrentSnapshotSeq: current.CurrentSnapshotSeq,
		}

		manifestData, err := json.Marshal(nextManifest)
		if err != nil {
			return fmt.Errorf("encode manifest: %w", err)
		}

		if s.TestHookBeforeManifestCommit != nil {
			if err := s.TestHookBeforeManifestCommit(); err != nil {
				return fmt.Errorf("manifest commit hook: %w", err)
			}
		}

		_, err = s.PutIfMatch(
			ctx,
			"index.json",
			manifestData,
			etag,
		)

		if err != nil {
			if isPreconditionFailed(err) {
				continue
			}

			return fmt.Errorf("commit manifest: %w", err)
		}

		return nil
	}
}

func (s *S3Storage) List(
	ctx context.Context,
	prefix string,
) ([]string, error) {
	var keys []string

	paginator := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(prefix),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("list %q: %w", prefix, err)
		}
		for _, obj := range page.Contents {
			if obj.Key != nil {
				keys = append(keys, *obj.Key)
			}
		}
	}

	return keys, nil
}
