package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"corpstore/internal/storage/meta"
)

// JWTMiddleware validates Authorization: Bearer <token>, checks the user exists and
// stores ownerID in gin context under key "ownerID".
func JWTMiddleware(secret []byte, m meta.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing Authorization header"})
			return
		}
		parts := strings.Fields(authHeader)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid Authorization header"})
			return
		}
		tokenStr := parts[1]

		claims := &jwt.RegisteredClaims{}
		tok, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
			// use HMAC
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrTokenMalformed
			}
			return secret, nil
		})
		if err != nil || tok == nil || !tok.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}

		userID := claims.Subject
		if userID == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token missing subject"})
			return
		}

		// optional: check user exists in DB
		ok, err := m.UserExists(context.Background(), userID)
		if err != nil || !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid user in token"})
			return
		}

		c.Set("ownerID", userID)
		c.Next()
	}
}
