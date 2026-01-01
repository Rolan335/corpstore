package auth

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"corpstore/internal/storage/meta"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
)

type Service struct {
	meta      meta.Store
	jwtSecret []byte
	jwtTTL    time.Duration
}

func NewService(m meta.Store, secret string, ttl time.Duration) *Service {
	return &Service{meta: m, jwtSecret: []byte(secret), jwtTTL: ttl}
}

// Register creates a user with hashed password
func (s *Service) Register(ctx context.Context, username, password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return s.meta.CreateUser(ctx, username, string(hash))
}

// Login verifies credentials and returns JWT token
func (s *Service) Login(ctx context.Context, username, password string) (string, error) {
	id, hash, err := s.meta.GetUserByUsername(ctx, username)
	if err != nil {
		return "", ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return "", ErrInvalidCredentials
	}

	// create JWT
	claims := jwt.RegisteredClaims{
		Subject:   id,
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.jwtTTL)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(s.jwtSecret)
	if err != nil {
		return "", err
	}
	return signed, nil
}

// ParseToken validates token and returns claims
func (s *Service) ParseToken(ctx context.Context, tokenStr string) (*jwt.RegisteredClaims, error) {
	claims := &jwt.RegisteredClaims{}
	tok, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrTokenMalformed
		}
		return s.jwtSecret, nil
	})
	if err != nil || tok == nil || !tok.Valid {
		return nil, errors.New("invalid token")
	}
	if claims.Subject == "" {
		return nil, errors.New("token missing subject")
	}
	return claims, nil
}

// Helper to load secret from env if not provided
func SecretFromEnv() string {
	if s := os.Getenv("AUTH_JWT_SECRET"); s != "" {
		return s
	}
	return "dev-secret"
}
