package main

import (
	"log"

	"github.com/home-storage/api/internal/cache"
	"github.com/home-storage/api/internal/config"
	"github.com/home-storage/api/internal/database"
	"github.com/home-storage/api/internal/handler"
	"github.com/home-storage/api/internal/repository"
	"github.com/home-storage/api/internal/router"
)

func main() {
	cfg := config.Load()
	db := database.Init(cfg)

	dbRepo := repository.NewDBRepository(db)

	var repo handler.Repository
	redisClient, err := cache.New(cfg)
	if err != nil {
		log.Printf("Redis unavailable, running without cache: %v", err)
		repo = dbRepo
	} else {
		log.Println("Redis connected, caching enabled")
		repo = repository.NewCachedRepository(dbRepo, redisClient)
	}

	r := router.Setup(handler.New(repo))
	log.Printf("Server starting on :%s", cfg.ServerPort)
	if err = r.Run(":" + cfg.ServerPort); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
