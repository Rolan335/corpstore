package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"corpstore/internal/auth"
	"corpstore/internal/handlers"
	"corpstore/internal/storage/db"
	localstorage "corpstore/internal/storage/local"
	storage_db "corpstore/internal/storage/storage_db"
	"corpstore/internal/telegram"
)

func main() {
	// try to load .env if present
	_ = godotenv.Load("./.env")

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
		dsn = "postgres://corpuser:corppass@db:5432/corpstore?sslmode=disable"
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

	// auth service
	jwtSecret := auth.SecretFromEnv()
	authSvc := auth.NewService(dbPool, jwtSecret, 24*time.Hour)
	// wire auth service into handlers
	h.SetAuth(authSvc)

	// try initialize telegram bot (optional)
	bot, err := telegram.NewBotFromEnv(h, authSvc)
	if err != nil {
		log.Printf("failed to init telegram bot: %v", err)
	} else {
		// start polling in background
		go bot.StartPolling(context.Background())
		log.Printf("telegram bot polling started")
	}

	healthHandler := handlers.NewHealthHandler(dbPool)

	// init health/readiness endpoint
	rHealth := gin.Default()
	rHealth.GET("/live", healthHandler.Healthy)
	rHealth.GET("/ready", healthHandler.Ready)

	go func() {
		if err := rHealth.Run("127.0.0.1:8081"); err != nil {
			log.Fatalf("health server error: %v", err)
		}
	}()

	r := gin.Default()

	// auth endpoints
	r.POST("/reg", h.CreateUser)
	r.POST("/login", h.Login)

	// protected group
	authMw := auth.JWTMiddleware([]byte(jwtSecret), dbPool)
	grp := r.Group("/")
	grp.Use(authMw)
	grp.POST("/files", h.UploadFiles)
	grp.GET("/files/:id", h.GetFile)
	grp.GET("/files", h.ListFiles)

	addr := ":8080"
	log.Printf("listening on %s, storing files in %s", addr, dataDir)

	if err := r.Run(addr); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
