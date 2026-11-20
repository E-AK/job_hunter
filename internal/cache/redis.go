package cache

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisBackend struct {
	client *redis.Client
}

func NewRedisBackend(client *redis.Client) *RedisBackend {
	return &RedisBackend{client: client}
}

func (r *RedisBackend) Get(ctx context.Context, key string) (string, error) {
	value, err := r.client.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return "", ErrNotFound
	}
	return value, err
}

func (r *RedisBackend) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	return r.client.Set(ctx, key, value, ttl).Err()
}

func (r *RedisBackend) Incr(ctx context.Context, key string) error {
	return r.client.Incr(ctx, key).Err()
}

func (r *RedisBackend) GetIfVersion(ctx context.Context, versionKey, dataKey, expectedVersion string) (string, error) {
	value, err := r.client.Eval(ctx, `
local version = redis.call('GET', KEYS[1]) or '0'
if version ~= ARGV[1] then
  return false
end
return redis.call('GET', KEYS[2])
`, []string{versionKey, dataKey}, expectedVersion).Result()
	if errors.Is(err, redis.Nil) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	result, ok := value.(string)
	if !ok {
		return "", ErrNotFound
	}
	return result, nil
}
