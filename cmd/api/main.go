package main

import (
	"log"

	"github.com/home-storage/api/internal/config"
	"github.com/home-storage/api/internal/database"
	"github.com/home-storage/api/internal/handler"
	"github.com/home-storage/api/internal/repository"
	"github.com/home-storage/api/internal/router"
)

func main() {
	cfg := config.Load()
	db := database.Init(cfg)
	repo := repository.NewDBRepository(db)
	r := router.Setup(handler.New(repo))
	log.Printf("Server starting on :%s", cfg.ServerPort)
	if err := r.Run(":" + cfg.ServerPort); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
