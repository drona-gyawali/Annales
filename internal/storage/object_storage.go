package storage

import "context"

type ObjectStorage interface {
	Get(ctx context.Context, key string) ([]byte, string, error)
	Put(ctx context.Context, key string, data []byte) (string, error)
	PutIfMatch(ctx context.Context, key string, data []byte, etag string) (string, error)
	PutIfNoneMatch(ctx context.Context, key string, data []byte) (string, error)
	Delete(ctx context.Context, key string) error
}
