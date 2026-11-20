package cache

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"
)

type memoryBackend struct {
	mu       sync.Mutex
	values   map[string]memoryValue
	failures map[string]error
}

type memoryValue struct {
	value     string
	expiresAt time.Time
}

func newMemoryBackend() *memoryBackend {
	return &memoryBackend{values: make(map[string]memoryValue), failures: make(map[string]error)}
}

func (b *memoryBackend) Get(_ context.Context, key string) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := b.failures["get"]; err != nil {
		return "", err
	}
	value, ok := b.values[key]
	if !ok || (!value.expiresAt.IsZero() && time.Now().After(value.expiresAt)) {
		delete(b.values, key)
		return "", ErrNotFound
	}
	return value.value, nil
}

func (b *memoryBackend) Set(_ context.Context, key, value string, ttl time.Duration) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := b.failures["set"]; err != nil {
		return err
	}
	b.values[key] = memoryValue{value: value, expiresAt: time.Now().Add(ttl)}
	return nil
}

func (b *memoryBackend) Incr(_ context.Context, key string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := b.failures["incr"]; err != nil {
		return err
	}
	value := b.values[key]
	if value.value == "" {
		value.value = "0"
	}
	var version int
	if _, err := fmt.Sscanf(value.value, "%d", &version); err != nil {
		return err
	}
	value.value = strconv.Itoa(version + 1)
	b.values[key] = value
	return nil
}

func (b *memoryBackend) GetIfVersion(_ context.Context, versionKey, dataKey, expectedVersion string) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := b.failures["get_if_version"]; err != nil {
		return "", err
	}
	current := b.values[versionKey].value
	if current == "" {
		current = "0"
	}
	if current != expectedVersion {
		return "", ErrNotFound
	}
	value, ok := b.values[dataKey]
	if !ok || time.Now().After(value.expiresAt) {
		delete(b.values, dataKey)
		return "", ErrNotFound
	}
	return value.value, nil
}

func TestResultCacheHitMissAndArguments(t *testing.T) {
	ctx := context.Background()
	backend := newMemoryBackend()
	cache := New(backend, time.Minute)
	args := map[string]any{"limit": float64(10)}

	if _, hit, err := cache.Get(ctx, "get_recent_vacancies", args); err != nil || hit {
		t.Fatalf("initial Get = (hit %v, err %v), want a miss", hit, err)
	}
	if err := cache.Set(ctx, "get_recent_vacancies", args, map[string]any{"count": 2}); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, hit, err := cache.Get(ctx, "get_recent_vacancies", args)
	if err != nil || !hit {
		t.Fatalf("Get = (hit %v, err %v), want a hit", hit, err)
	}
	if got.(map[string]any)["count"] != float64(2) {
		t.Fatalf("cached result = %#v", got)
	}
	if _, hit, err := cache.Get(ctx, "get_recent_vacancies", map[string]any{"limit": float64(20)}); err != nil || hit {
		t.Fatalf("different arguments = (hit %v, err %v), want a miss", hit, err)
	}
	if _, hit, err := cache.Get(ctx, "list_resumes", args); err != nil || hit {
		t.Fatalf("different tool = (hit %v, err %v), want a miss", hit, err)
	}
}

func TestResultCacheTTLAndInvalidation(t *testing.T) {
	ctx := context.Background()
	backend := newMemoryBackend()
	cache := New(backend, 5*time.Millisecond)
	args := map[string]any{}
	if err := cache.Set(ctx, "list_resumes", args, []any{"resume"}); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if _, hit, err := cache.Get(ctx, "list_resumes", args); err != nil || !hit {
		t.Fatalf("Get before expiry = (hit %v, err %v)", hit, err)
	}
	time.Sleep(10 * time.Millisecond)
	if _, hit, err := cache.Get(ctx, "list_resumes", args); err != nil || hit {
		t.Fatalf("Get after expiry = (hit %v, err %v), want a miss", hit, err)
	}

	if err := cache.Set(ctx, "list_resumes", args, []any{"resume"}); err != nil {
		t.Fatalf("Set before invalidation: %v", err)
	}
	if err := cache.Invalidate(ctx); err != nil {
		t.Fatalf("Invalidate: %v", err)
	}
	if _, hit, err := cache.Get(ctx, "list_resumes", args); err != nil || hit {
		t.Fatalf("Get after invalidation = (hit %v, err %v), want a miss", hit, err)
	}
}

func TestResultCachePropagatesBackendErrors(t *testing.T) {
	ctx := context.Background()
	backend := newMemoryBackend()
	cache := New(backend, time.Minute)
	wantErr := errors.New("Redis unavailable")

	backend.failures["get"] = wantErr
	if _, _, err := cache.Get(ctx, "tool", map[string]any{}); !errors.Is(err, wantErr) {
		t.Fatalf("Get error = %v, want %v", err, wantErr)
	}
	delete(backend.failures, "get")

	backend.failures["set"] = wantErr
	if err := cache.Set(ctx, "tool", map[string]any{}, "result"); !errors.Is(err, wantErr) {
		t.Fatalf("Set error = %v, want %v", err, wantErr)
	}
	delete(backend.failures, "set")

	backend.failures["incr"] = wantErr
	if err := cache.Invalidate(ctx); !errors.Is(err, wantErr) {
		t.Fatalf("Invalidate error = %v, want %v", err, wantErr)
	}
}
