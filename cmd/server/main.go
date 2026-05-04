package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"time"

	"stock-market/internal/httpapi"
	"stock-market/internal/service"
	"stock-market/internal/store/postgres"
)

func main() {
	port := envOrDefault("PORT", "8080")
	instanceID := envOrDefault("INSTANCE_ID", "api")
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is required")
	}

	db, err := postgres.Connect(dsn)
	if err != nil {
		log.Fatalf("database connection failed: %v", err)
	}
	defer db.Close()

	if err := postgres.InitSchema(context.Background(), db); err != nil {
		log.Fatalf("schema initialization failed: %v", err)
	}

	repo := postgres.New(db)
	svc := service.New(repo)
	handler := httpapi.New(svc, instanceID)

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	log.Printf("instance=%s listening on :%s", instanceID, port)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("server failed: %v", err)
	}
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
