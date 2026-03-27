package idempotency

import (
	"context"
	"errors"
	"time"
)

var ErrKeyNotFound = errors.New("idempotency key not found")


type Store interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value string, ttl time.Duration) error
	SetNX(ctx context.Context, key string, value string, ttl time.Duration) (bool, error)
	Del(ctx context.Context, key string) error
}