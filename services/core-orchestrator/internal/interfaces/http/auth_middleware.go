package http

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"core-orchestrator/internal/application/credentials"
	"core-orchestrator/internal/domain"
)

type authContextKey string

const userContextKey authContextKey = "auth_user"

type JWTMiddleware struct {
	service *credentials.AuthService
}

func NewJWTMiddleware(service *credentials.AuthService) *JWTMiddleware {
	return &JWTMiddleware{service: service}
}

func (m *JWTMiddleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorizationHeader := strings.TrimSpace(r.Header.Get("Authorization"))
		if authorizationHeader == "" {
			writeJSONError(w, http.StatusUnauthorized, "missing authorization header")
			return
		}

		const bearerPrefix = "Bearer "
		if !strings.HasPrefix(authorizationHeader, bearerPrefix) {
			writeJSONError(w, http.StatusUnauthorized, "invalid authorization schema")
			return
		}

		tokenString := strings.TrimSpace(strings.TrimPrefix(authorizationHeader, bearerPrefix))
		if tokenString == "" {
			writeJSONError(w, http.StatusUnauthorized, "missing token")
			return
		}

		user, err := m.service.ValidateToken(r.Context(), tokenString)
		if err != nil {
			if errors.Is(err, credentials.ErrUnauthorized) {
				writeJSONError(w, http.StatusUnauthorized, "invalid or expired token")
				return
			}

			writeJSONError(w, http.StatusInternalServerError, "internal error")
			return
		}

		ctx := context.WithValue(r.Context(), userContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func UserFromContext(ctx context.Context) (*domain.AuthUser, bool) {
	user, ok := ctx.Value(userContextKey).(*domain.AuthUser)
	return user, ok
}
