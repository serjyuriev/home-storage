package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/home-storage/api/internal/cache"
	"github.com/home-storage/api/internal/config"
	"github.com/home-storage/api/internal/database"
	"github.com/home-storage/api/internal/handler"
	"github.com/home-storage/api/internal/repository"
	"github.com/home-storage/api/internal/router"
	"github.com/redis/go-redis/v9"
)

func main() {
	cfg := config.Load()
	db := database.Init(cfg)

	dbRepo := repository.NewDBRepository(db)

	var (
		repo        handler.Repository = dbRepo
		redisClient *redis.Client
	)
	if rc, err := cache.New(cfg); err != nil {
		log.Printf("Redis unavailable, running without cache: %v", err)
	} else {
		log.Println("Redis connected, caching enabled")
		redisClient = rc
		repo = repository.NewCachedRepository(dbRepo, rc)
	}

	engine := router.Setup(handler.New(repo))
	srv := &http.Server{
		Addr:    ":" + cfg.ServerPort,
		Handler: engine,
	}

	go func() {
		log.Printf("Server starting on :%s", cfg.ServerPort)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("HTTP shutdown error: %v", err)
	}

	if redisClient != nil {
		if err := redisClient.Close(); err != nil {
			log.Printf("Redis close error: %v", err)
		}
	}

	if sqlDB, err := db.DB(); err == nil {
		if err = sqlDB.Close(); err != nil {
			log.Printf("DB close error: %v", err)
		}
	}

	log.Println("Server stopped")
}
