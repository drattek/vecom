package http

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	rolesApp "core-orchestrator/internal/application/roles"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

// RolesHandler sirve Configuración → Roles (CRUD de roles con sus permisos) y
// el catálogo de permisos asignables. Ver ADR 0009.
type RolesHandler struct {
	service *rolesApp.RoleService
}

type roleRequest struct {
	Name          string  `json:"name"`
	Description   string  `json:"description"`
	PermissionIDs []int64 `json:"permissionIds"`
}

func NewRolesHandler(service *rolesApp.RoleService) *RolesHandler {
	return &RolesHandler{service: service}
}

func (h *RolesHandler) ListRoles(w http.ResponseWriter, r *http.Request) {
	roles, err := h.service.ListRoles(r.Context())
	if err != nil {
		h.fail(w, "list roles", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"roles": roles})
}

func (h *RolesHandler) ListPermissions(w http.ResponseWriter, r *http.Request) {
	perms, err := h.service.ListPermissions(r.Context())
	if err != nil {
		h.fail(w, "list permissions", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"permissions": perms})
}

func (h *RolesHandler) GetRole(w http.ResponseWriter, r *http.Request) {
	id, ok := parseRoleID(w, r)
	if !ok {
		return
	}
	role, err := h.service.GetRole(r.Context(), id)
	if err != nil {
		h.fail(w, "get role", err)
		return
	}
	writeJSON(w, http.StatusOK, role)
}

func (h *RolesHandler) CreateRole(w http.ResponseWriter, r *http.Request) {
	var req roleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	actor, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	role, err := h.service.CreateRole(r.Context(), rolesApp.RoleInput{
		Name: req.Name, Description: req.Description, PermissionIDs: req.PermissionIDs, ActorID: actor.ID,
	})
	if err != nil {
		h.fail(w, "create role", err)
		return
	}
	writeJSON(w, http.StatusCreated, role)
}

func (h *RolesHandler) UpdateRole(w http.ResponseWriter, r *http.Request) {
	id, ok := parseRoleID(w, r)
	if !ok {
		return
	}
	var req roleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	actor, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	role, err := h.service.UpdateRole(r.Context(), id, rolesApp.RoleInput{
		Name: req.Name, Description: req.Description, PermissionIDs: req.PermissionIDs, ActorID: actor.ID,
	})
	if err != nil {
		h.fail(w, "update role", err)
		return
	}
	writeJSON(w, http.StatusOK, role)
}

func (h *RolesHandler) DeleteRole(w http.ResponseWriter, r *http.Request) {
	id, ok := parseRoleID(w, r)
	if !ok {
		return
	}
	actor, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if err := h.service.DeleteRole(r.Context(), id, actor.ID); err != nil {
		h.fail(w, "delete role", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *RolesHandler) fail(w http.ResponseWriter, op string, err error) {
	switch {
	case errors.Is(err, rolesApp.ErrInvalidRolePayload):
		writeJSONError(w, http.StatusBadRequest, "el nombre del rol es obligatorio")
	case errors.Is(err, mysqlInfra.ErrPermissionInvalid):
		writeJSONError(w, http.StatusBadRequest, "uno de los permisos no existe")
	case errors.Is(err, mysqlInfra.ErrRoleNotFound):
		writeJSONError(w, http.StatusNotFound, "role not found")
	case errors.Is(err, mysqlInfra.ErrRoleHasUsers):
		writeJSONError(w, http.StatusConflict, "el rol tiene usuarios asignados; reasígnalos antes de eliminarlo")
	default:
		log.Printf("roles handler: %s: %v", op, err)
		writeJSONError(w, http.StatusInternalServerError, "internal error")
	}
}

func parseRoleID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid id")
		return 0, false
	}
	return id, true
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
