package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"

	"corpstore/internal/auth"
	"corpstore/internal/users"
)

// Auth handles user registration and login.
type Auth struct {
	svc   *auth.Service
	users users.Repository
}

func NewAuth(svc *auth.Service, usersRepo users.Repository) *Auth {
	return &Auth{svc: svc, users: usersRepo}
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

// EnsureTelegramUser creates a user for a telegram id if missing.
func (u *Auth) EnsureTelegramUser(ctx context.Context, tgID int64, tgUsername string) (string, error) {
	tgUsername = strings.TrimSpace(tgUsername)
	if tgUsername != "" && tgUsername[0] != '@' {
		tgUsername = "@" + tgUsername
	}
	if tgUsername != "" {
		rec, err := u.users.GetByUsername(ctx, tgUsername)
		if err == nil {
			return rec.ID, nil
		}
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return "", err
		}
	}

	fallback := fmt.Sprintf("tg:%d", tgID)
	rec, err := u.users.GetByUsername(ctx, fallback)
	if err == nil {
		if tgUsername != "" {
			_ = u.users.UpdateUsername(ctx, rec.ID, tgUsername)
		}
		return rec.ID, nil
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	if tgUsername != "" {
		return u.users.Create(ctx, tgUsername, "")
	}
	return u.users.Create(ctx, fallback, "")
}
