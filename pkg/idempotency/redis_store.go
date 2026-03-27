package idempotency

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisStore struct {
	client *redis.Client
}

func NewRedisStore(client *redis.Client) *RedisStore {
	return &RedisStore{client: client}
}

func (s *RedisStore) Get(ctx context.Context, key string) (string, error) {
	v, err := s.client.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return "", ErrKeyNotFound
	}
	if err != nil {
		return "", err
	}
	return v, nil
}

func (s *RedisStore) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	return s.client.Set(ctx, key, value, ttl).Err()
}

func (s *RedisStore) SetNX(ctx context.Context, key string, value string, ttl time.Duration) (bool, error) {
	return s.client.SetNX(ctx, key, value, ttl).Result()
}

func (s *RedisStore) Del(ctx context.Context, key string) error {
	return s.client.Del(ctx, key).Err()
}