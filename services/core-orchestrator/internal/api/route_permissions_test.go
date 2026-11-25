package api

import (
	"net/http"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

// routerRoutes arma el router real con handlers vacíos (solo se toman sus
// method values, nunca se invocan) y devuelve "MÉTODO patrón" de cada ruta.
func routerRoutes(t *testing.T) []string {
	t.Helper()

	fn := reflect.ValueOf(NewRouter)
	args := make([]reflect.Value, fn.Type().NumIn())
	for i := range args {
		args[i] = reflect.New(fn.Type().In(i).Elem())
	}
	router, ok := fn.Call(args)[0].Interface().(chi.Routes)
	if !ok {
		t.Fatal("NewRouter no devolvió chi.Routes")
	}

	var routes []string
	err := chi.Walk(router, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		routes = append(routes, method+" "+strings.TrimSuffix(route, "/"))
		return nil
	})
	if err != nil {
		t.Fatalf("chi.Walk: %v", err)
	}
	return routes
}

// Toda clave de routePermissions debe corresponder a una ruta real: un typo
// en la política dejaría la ruta como solo-admin sin que nadie lo note.
func TestRoutePermissionsMatchRouter(t *testing.T) {
	registered := map[string]bool{}
	for _, r := range routerRoutes(t) {
		registered[r] = true
	}

	var orphans []string
	for key := range routePermissions {
		if !registered[key] {
			orphans = append(orphans, key)
		}
	}
	sort.Strings(orphans)
	if len(orphans) > 0 {
		t.Errorf("claves de routePermissions sin ruta en el router:\n  %s", strings.Join(orphans, "\n  "))
	}
}

// Informativo: rutas protegidas sin política (solo admin). Se revisa con -v.
func TestAdminOnlyRoutesListing(t *testing.T) {
	var adminOnly []string
	for _, r := range routerRoutes(t) {
		if _, ok := routePermissions[r]; ok {
			continue
		}
		if strings.Contains(r, "/api/login") || strings.Contains(r, "/meli_") ||
			strings.HasSuffix(r, " /") || strings.HasSuffix(r, " /health") {
			continue // públicas
		}
		adminOnly = append(adminOnly, r)
	}
	sort.Strings(adminOnly)
	t.Logf("rutas solo-admin (%d):\n  %s", len(adminOnly), strings.Join(adminOnly, "\n  "))
}
