package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

var ErrNotFound = errors.New("cache key not found")

const (
	cachePrefix = "job-hunter:mcp"
	versionKey  = cachePrefix + ":version"
)

type Backend interface {
	Get(context.Context, string) (string, error)
	Set(context.Context, string, string, time.Duration) error
	Incr(context.Context, string) error
	GetIfVersion(context.Context, string, string, string) (string, error)
}

type ResultCache struct {
	backend Backend
	ttl     time.Duration
}

func New(backend Backend, ttl time.Duration) *ResultCache {
	return &ResultCache{backend: backend, ttl: ttl}
}

func (c *ResultCache) Get(ctx context.Context, tool string, arguments any) (any, bool, error) {
	key, err := resultKey(tool, arguments)
	if err != nil {
		return nil, false, err
	}
	version, err := c.currentVersion(ctx)
	if err != nil {
		return nil, false, err
	}
	data, err := c.backend.GetIfVersion(ctx, versionKey, key+":"+version, version)
	if errors.Is(err, ErrNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	var result any
	if err := json.Unmarshal([]byte(data), &result); err != nil {
		return nil, false, fmt.Errorf("decode cached MCP result: %w", err)
	}
	return result, true, nil
}

func (c *ResultCache) Set(ctx context.Context, tool string, arguments, result any) error {
	key, err := resultKey(tool, arguments)
	if err != nil {
		return err
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("encode MCP result for cache: %w", err)
	}
	version, err := c.currentVersion(ctx)
	if err != nil {
		return err
	}
	if err := c.backend.Set(ctx, key+":"+version, string(encoded), c.ttl); err != nil {
		return err
	}
	return nil
}

func (c *ResultCache) Invalidate(ctx context.Context) error {
	return c.backend.Incr(ctx, versionKey)
}

func (c *ResultCache) currentVersion(ctx context.Context) (string, error) {
	version, err := c.backend.Get(ctx, versionKey)
	if errors.Is(err, ErrNotFound) {
		return "0", nil
	}
	if err != nil {
		return "", err
	}
	return version, nil
}

func resultKey(tool string, arguments any) (string, error) {
	encoded, err := json.Marshal(arguments)
	if err != nil {
		return "", fmt.Errorf("encode MCP tool arguments for cache: %w", err)
	}
	sum := sha256.Sum256(append(append([]byte(tool), 0), encoded...))
	return cachePrefix + ":result:" + hex.EncodeToString(sum[:]), nil
}
