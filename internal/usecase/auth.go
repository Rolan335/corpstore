package usecase

import (
	"context"
	"errors"
	"strings"

	"github.com/golang-jwt/jwt/v5"

	"corpstore/internal/auth"
)

// Auth handles user registration and login.
type Auth struct {
	svc *auth.Service
}

func NewAuth(svc *auth.Service) *Auth {
	return &Auth{svc: svc}
}

func (u *Auth) Register(ctx context.Context, username, password string) (string, error) {
	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return "", errors.New("username and password required")
	}
	return u.svc.Register(ctx, username, password)
}

func (u *Auth) Login(ctx context.Context, username, password string) (string, error) {
	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return "", errors.New("username and password required")
	}
	return u.svc.Login(ctx, username, password)
}

func (u *Auth) ParseToken(ctx context.Context, token string) (*jwt.RegisteredClaims, error) {
	return u.svc.ParseToken(ctx, token)
}
