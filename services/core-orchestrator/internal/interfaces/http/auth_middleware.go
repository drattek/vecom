package http

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

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

// AuthenticatedOnly marca una ruta que solo exige sesión válida (sin permiso).
const AuthenticatedOnly = "*"

// RoutePolicy mapea "MÉTODO /patrón/chi" al conjunto de permisos que dan
// acceso (basta uno). Una ruta ausente del mapa es solo para admin
// (default-deny), por eso las rutas temporales, de migración y de debug no
// necesitan entrada. Ver ADR 0009.
type RoutePolicy map[string][]string

// RequireRoutePermission aplica RoutePolicy sobre el patrón de la ruta ya
// resuelta por chi. Debe ir después de RequireAuth. Admin siempre pasa.
func (m *JWTMiddleware) RequireRoutePermission(policy RoutePolicy) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := UserFromContext(r.Context())
			if !ok {
				writeJSONError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			if user.IsAdmin() {
				next.ServeHTTP(w, r)
				return
			}

			key := r.Method + " " + chi.RouteContext(r.Context()).RoutePattern()
			required, mapped := policy[key]
			if mapped && len(required) == 1 && required[0] == AuthenticatedOnly {
				next.ServeHTTP(w, r)
				return
			}
			if mapped && user.HasAnyPermission(required...) {
				next.ServeHTTP(w, r)
				return
			}

			writeJSONError(w, http.StatusForbidden, "no tienes permiso para realizar esta acción")
		})
	}
}
