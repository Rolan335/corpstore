package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"corpstore/internal/auth"
	"corpstore/internal/files"
	"corpstore/internal/filestore/local"
	"corpstore/internal/handlers"
	"corpstore/internal/telegram"
	"corpstore/internal/users"
	"corpstore/internal/usecase"
)

//TODO: Сделать удаление файлов и меты из бд.
//TODO: Изменение пользователя
//TODO: Комменты на русском
//TODO: Обмен файлами между зарегаными юзерами
//TODO: Шифровать файлы на сервере как-то

func main() {
	// try to load .env if present
	_ = godotenv.Load("./.env")

	// storage directory
	dataDir := "./data"
	if v := os.Getenv("CORPSTORE_DATA_DIR"); v != "" {
		dataDir = v
	}

	st, err := local.New(dataDir)
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

	dbPool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatalf("failed to connect db: %v", err)
	}
	defer dbPool.Close()

	usersRepo := users.NewPostgresRepository(dbPool)
	filesRepo := files.NewPostgresRepository(dbPool)
	filesSvc := files.NewService(st, filesRepo)

	authProvider := auth.NewProvider(usersRepo, auth.ConfigFromEnv())
	filesUC := usecase.NewFiles(filesSvc, usersRepo)
	authUC := usecase.NewAuth(authProvider.Service)
	h := handlers.NewHandler(filesUC, authUC)

	// try initialize telegram bot (optional)
	tgCfg, err := telegram.ConfigFromEnv()
	if err != nil {
		log.Printf("failed to init telegram bot: %v", err)
	} else {
		tgProvider, err := telegram.NewProvider(tgCfg, authUC, filesUC)
		if err != nil {
			log.Printf("failed to init telegram bot: %v", err)
		} else {
			// start polling in background
			go tgProvider.Bot.StartPolling(context.Background())
			log.Printf("telegram bot polling started")
		}
	}

	healthHandler := handlers.NewHealthHandler(dbPool)

	// init health/readiness endpoint
	rHealth := gin.Default()
	rHealth.GET("/live", healthHandler.Healthy)
	rHealth.GET("/ready", healthHandler.Ready)

	go func() {
		if err := rHealth.Run(":8081"); err != nil {
			log.Fatalf("health server error: %v", err)
		}
	}()

	r := gin.Default()

	// auth endpoints
	r.POST("/reg", h.CreateUser)
	r.POST("/login", h.Login)

	// protected group
	authMw := authProvider.Middleware
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
