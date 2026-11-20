package config

import "os"

type Config struct {
	Addr, DatabaseURL, AuthToken, HHUserAgent, SuperJobAppID, SuperJobToken, SuperJobIzhevskTownID string
	DefaultLimit                                                                                   int
}

func Load() Config {
	return Config{Addr: env("HTTP_ADDR", ":8092"), DatabaseURL: env("DATABASE_URL", "postgres://jobhunter:change-me@localhost:5432/jobhunter?sslmode=disable"), AuthToken: os.Getenv("MCP_AUTH_TOKEN"), HHUserAgent: env("HH_USER_AGENT", "JobHunterMCP/0.1"), SuperJobAppID: os.Getenv("SUPERJOB_APP_ID"), SuperJobToken: os.Getenv("SUPERJOB_ACCESS_TOKEN"), SuperJobIzhevskTownID: os.Getenv("SUPERJOB_IZHEVSK_TOWN_ID"), DefaultLimit: 30}
}
func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
