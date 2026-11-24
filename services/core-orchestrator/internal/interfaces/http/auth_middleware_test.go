package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"core-orchestrator/internal/domain"
)

func TestRequireRoutePermission(t *testing.T) {
	policy := RoutePolicy{
		"GET /api/brands":         {"brands.view", "products.view"},
		"PUT /api/brands/{id}":    {"brands.edit"},
		"GET /api/me":             {AuthenticatedOnly},
		"DELETE /api/brands/{id}": {"brands.delete"},
	}

	mw := (&JWTMiddleware{}).RequireRoutePermission(policy)
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

	build := func(user *domain.AuthUser) http.Handler {
		r := chi.NewRouter()
		r.Group(func(g chi.Router) {
			g.Use(func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
					ctx := context.WithValue(req.Context(), userContextKey, user)
					next.ServeHTTP(w, req.WithContext(ctx))
				})
			})
			g.Use(mw)
			g.Get("/api/brands", ok)
			g.Put("/api/brands/{id}", ok)
			g.Delete("/api/brands/{id}", ok)
			g.Get("/api/me", ok)
			g.Get("/api/secret", ok) // sin política → solo admin
		})
		return r
	}

	cases := []struct {
		name   string
		user   *domain.AuthUser
		method string
		path   string
		want   int
	}{
		{"admin entra a todo", &domain.AuthUser{Role: "admin"}, "GET", "/api/secret", 200},
		{"admin sin permisos entra a ruta con política", &domain.AuthUser{Role: "admin"}, "PUT", "/api/brands/7", 200},
		{"permiso exacto", &domain.AuthUser{Role: "user", Permissions: []string{"brands.edit"}}, "PUT", "/api/brands/7", 200},
		{"cualquiera de la lista", &domain.AuthUser{Role: "user", Permissions: []string{"products.view"}}, "GET", "/api/brands", 200},
		{"permiso insuficiente", &domain.AuthUser{Role: "user", Permissions: []string{"brands.view"}}, "PUT", "/api/brands/7", 403},
		{"sin permisos", &domain.AuthUser{Role: "user", Permissions: []string{}}, "DELETE", "/api/brands/7", 403},
		{"solo autenticado", &domain.AuthUser{Role: "user", Permissions: []string{}}, "GET", "/api/me", 200},
		{"ruta sin política es solo admin", &domain.AuthUser{Role: "user", Permissions: []string{"brands.view"}}, "GET", "/api/secret", 403},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			build(tc.user).ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))
			if rec.Code != tc.want {
				t.Fatalf("%s %s: got %d, want %d", tc.method, tc.path, rec.Code, tc.want)
			}
		})
	}
}
