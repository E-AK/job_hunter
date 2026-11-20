package config

import "os"

type Config struct {
	Addr, DatabaseURL, AuthToken                                                        string
	SuperJobAppSecret, SuperJobClientID, SuperJobAuthorizationCode, SuperJobRedirectURI string
	SuperJobToken, SuperJobIzhevskTownID                                                string
	RedisAddr, RedisPassword, RedisDB, CacheTTL                                         string
	DefaultLimit                                                                        int
}

func Load() Config {
	return Config{
		Addr: env("HTTP_ADDR", ":8092"), DatabaseURL: env("DATABASE_URL", "postgres://jobhunter:change-me@localhost:5432/jobhunter?sslmode=disable"),
		AuthToken:                 os.Getenv("MCP_AUTH_TOKEN"),
		SuperJobAppSecret:         env("SUPERJOB_APP_SECRET", os.Getenv("SUPERJOB_APP_ID")),
		SuperJobClientID:          os.Getenv("SUPERJOB_CLIENT_ID"),
		SuperJobAuthorizationCode: os.Getenv("SUPERJOB_AUTHORIZATION_CODE"),
		SuperJobRedirectURI:       os.Getenv("SUPERJOB_REDIRECT_URI"),
		SuperJobToken:             os.Getenv("SUPERJOB_ACCESS_TOKEN"),
		SuperJobIzhevskTownID:     os.Getenv("SUPERJOB_IZHEVSK_TOWN_ID"),
		RedisAddr:                 env("REDIS_ADDR", "localhost:6379"), RedisPassword: os.Getenv("REDIS_PASSWORD"),
		RedisDB: env("REDIS_DB", "0"), CacheTTL: env("CACHE_TTL", "5m"), DefaultLimit: 30,
	}
}
func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
