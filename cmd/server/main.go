package main

import (
	"context"
	"job-hunter-mcp/internal/config"
	"job-hunter-mcp/internal/mcp"
	"job-hunter-mcp/internal/storage"
	"log"
	"net/http"
	"os"
	"time"
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
	srv := &mcp.Server{Store: st, Token: c.AuthToken, HHUserAgent: c.HHUserAgent, SJAppID: c.SuperJobAppID, SJToken: c.SuperJobToken, SJIzhevskTownID: c.SuperJobIzhevskTownID}
	mux := http.NewServeMux()
	mux.Handle("/", srv.Handler())
	log.Printf("job-hunter-mcp listening on %s", c.Addr)
	if err = http.ListenAndServe(c.Addr, mux); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}
