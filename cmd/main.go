package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/gin-gonic/gin"

	"corpstore/internal/handlers"
	"corpstore/internal/storage/db"
	localstorage "corpstore/internal/storage/local"
	"corpstore/internal/storage/storage_db"
)

func main() {
	// storage directory
	dataDir := "./data"
	if v := os.Getenv("CORPSTORE_DATA_DIR"); v != "" {
		dataDir = v
	}

	st, err := localstorage.NewLocalStorage(dataDir)
	if err != nil {
		log.Fatalf("failed to initialize storage: %v", err)
	}

	// initialize DB
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://corpuser:corppass@localhost:5432/corpstore?sslmode=disable"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dbPool, err := db.NewDB(ctx, dsn)
	if err != nil {
		log.Fatalf("failed to connect db: %v", err)
	}
	defer dbPool.Close()

	// create storage that composes file bytes store and meta DB
	composed := storage_db.NewDBStorage(st, dbPool)

	h := handlers.NewHandler(composed, dbPool)

	r := gin.Default()

	// Upload endpoint
	r.POST("/files", h.UploadFilesGin)
	// Download endpoint
	r.GET("/files/:id", h.GetFileGin)
	// Create user
	r.POST("/users", h.CreateUserGin)

	addr := ":8080"
	log.Printf("listening on %s, storing files in %s", addr, dataDir)
	if err := r.Run(addr); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
