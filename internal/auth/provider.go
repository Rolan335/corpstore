package auth

import (
	"os"
	"time"

	"github.com/gin-gonic/gin"

	"corpstore/internal/users"
)

// Config holds auth module configuration.
type Config struct {
	JWTSecret string
	TokenTTL  time.Duration
}

func ConfigFromEnv() Config {
	secret := os.Getenv("AUTH_JWT_SECRET")
	if secret == "" {
		secret = "dev-secret"
	}
	return Config{
		JWTSecret: secret,
		TokenTTL:  24 * time.Hour,
	}
}

// Provider wires auth service and middleware.
type Provider struct {
	Service    *Service
	Middleware gin.HandlerFunc
}

func NewProvider(repo users.Repository, cfg Config) *Provider {
	svc := NewService(repo, cfg.JWTSecret, cfg.TokenTTL)
	return &Provider{
		Service:    svc,
		Middleware: JWTMiddleware(svc, repo),
	}
}
