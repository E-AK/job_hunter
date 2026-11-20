package main

import (
	"context"
	"fmt"
	"job-hunter-mcp/internal/cache"
	"job-hunter-mcp/internal/config"
	"job-hunter-mcp/internal/mcp"
	"job-hunter-mcp/internal/storage"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

func main() {
	c := config.Load()
	if len(c.AuthToken) < 32 {
		log.Fatal("MCP_AUTH_TOKEN is required and must contain at least 32 characters")
	}
	ctx := context.Background()
	var st *storage.Store
	var err error
	for i := 0; i < 30; i++ {
		st, err = storage.New(ctx, c.DatabaseURL)
		if err == nil {
			break
		}
		log.Printf("waiting for database: %v", err)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		log.Fatal(err)
	}
	defer st.DB.Close()
	redisDB, err := strconv.Atoi(c.RedisDB)
	if err != nil || redisDB < 0 {
		log.Fatalf("REDIS_DB must be a non-negative integer: %q", c.RedisDB)
	}
	cacheTTL, err := time.ParseDuration(c.CacheTTL)
	if err != nil || cacheTTL <= 0 {
		log.Fatalf("CACHE_TTL must be a positive duration: %q", c.CacheTTL)
	}
	redisClient := redis.NewClient(&redis.Options{Addr: c.RedisAddr, Password: c.RedisPassword, DB: redisDB})
	defer redisClient.Close()
	for i := 0; i < 30; i++ {
		if err = redisClient.Ping(ctx).Err(); err == nil {
			break
		}
		log.Printf("waiting for Redis: %v", err)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		log.Fatal(fmt.Errorf("connect to Redis: %w", err))
	}
	srv := &mcp.Server{
		Store: st, Cache: cache.New(cache.NewRedisBackend(redisClient), cacheTTL),
		Token: c.AuthToken, SJAppID: c.SuperJobAppID, SJToken: c.SuperJobToken, SJIzhevskTownID: c.SuperJobIzhevskTownID,
	}
	mux := http.NewServeMux()
	mux.Handle("/", srv.Handler())
	log.Printf("job-hunter-mcp listening on %s", c.Addr)
	if err = http.ListenAndServe(c.Addr, mux); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}
