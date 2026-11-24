package http

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"

	usersApp "core-orchestrator/internal/application/users"
)

// UserHandler serves the admin "Usuarios" page: GET /api/users (paginated
// list), GET/PUT /api/users/{id} (detalle y acceso: rol + excepciones de
// permisos) and POST /api/users (create, with password). El actor sale de
// UserFromContext (audita ecom_user_permissions.created_by).
type UserHandler struct {
	service *usersApp.UserService
}

type permissionOverrideRequest struct {
	PermissionID int64  `json:"permissionId"`
	Effect       string `json:"effect"`
}

type createUserRequest struct {
	Username  string                      `json:"username"`
	Password  string                      `json:"password"`
	IsAdmin   bool                        `json:"isAdmin"`
	RoleID    *int64                      `json:"roleId"`
	Overrides []permissionOverrideRequest `json:"overrides"`
}

type updateUserRequest struct {
	IsAdmin   bool                         `json:"isAdmin"`
	RoleID    *int64                       `json:"roleId"`
	IsActive  bool                         `json:"isActive"`
	Overrides *[]permissionOverrideRequest `json:"overrides"`
}

func NewUserHandler(service *usersApp.UserService) *UserHandler {
	return &UserHandler{service: service}
}

func (h *UserHandler) GetUsers(w http.ResponseWriter, r *http.Request) {
	offsetStr := r.URL.Query().Get("offset")
	pageSizeStr := r.URL.Query().Get("pageSize")

	offset := 0
	pageSize := 10

	if offsetStr != "" {
		if parsedOffset, err := strconv.Atoi(offsetStr); err == nil {
			offset = parsedOffset
		}
	}

	if pageSizeStr != "" {
		if parsedPageSize, err := strconv.Atoi(pageSizeStr); err == nil {
			pageSize = parsedPageSize
		}
	}

	result, err := h.service.GetPaginatedUsers(r.Context(), offset, pageSize)
	if err != nil {
		log.Printf("users handler: list users: %v", err)
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func (h *UserHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUserID(w, r)
	if !ok {
		return
	}
	detail, err := h.service.GetUser(r.Context(), id)
	if err != nil {
		h.fail(w, "get user", err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (h *UserHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req createUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	actor, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	user, err := h.service.CreateUser(r.Context(), usersApp.CreateUserInput{
		Username:  req.Username,
		Password:  req.Password,
		IsAdmin:   req.IsAdmin,
		RoleID:    req.RoleID,
		Overrides: toOverrides(req.Overrides),
		ActorID:   actor.ID,
	})
	if err != nil {
		h.fail(w, "create user", err)
		return
	}

	writeJSON(w, http.StatusCreated, user)
}

func (h *UserHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUserID(w, r)
	if !ok {
		return
	}
	var req updateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	actor, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	input := usersApp.UpdateUserInput{
		IsAdmin:  req.IsAdmin,
		RoleID:   req.RoleID,
		IsActive: req.IsActive,
		ActorID:  actor.ID,
	}
	if req.Overrides != nil {
		overrides := toOverrides(*req.Overrides)
		input.Overrides = &overrides
	}

	detail, err := h.service.UpdateUser(r.Context(), id, input)
	if err != nil {
		h.fail(w, "update user", err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (h *UserHandler) fail(w http.ResponseWriter, op string, err error) {
	switch {
	case errors.Is(err, usersApp.ErrInvalidUserPayload):
		writeJSONError(w, http.StatusBadRequest, "username and password are required")
	case errors.Is(err, usersApp.ErrPasswordTooShort):
		writeJSONError(w, http.StatusBadRequest, "password must be at least 8 characters")
	case errors.Is(err, usersApp.ErrInvalidRole):
		writeJSONError(w, http.StatusBadRequest, "el rol seleccionado no existe")
	case errors.Is(err, mysqlInfra.ErrPermissionInvalid):
		writeJSONError(w, http.StatusBadRequest, "uno de los permisos no existe")
	case errors.Is(err, usersApp.ErrLastAdmin):
		writeJSONError(w, http.StatusConflict, "no se puede quitar el último administrador activo")
	case errors.Is(err, usersApp.ErrSelfLockout):
		writeJSONError(w, http.StatusConflict, "no puedes quitarte tu propio acceso de administrador ni desactivarte")
	case errors.Is(err, mysqlInfra.ErrUserNotFound):
		writeJSONError(w, http.StatusNotFound, "user not found")
	case errors.Is(err, mysqlInfra.ErrUserUsernameAlreadyExists):
		writeJSONError(w, http.StatusConflict, "username already exists")
	default:
		log.Printf("users handler: %s: %v", op, err)
		writeJSONError(w, http.StatusInternalServerError, "internal error")
	}
}

func toOverrides(in []permissionOverrideRequest) []mysqlInfra.UserPermissionOverride {
	out := make([]mysqlInfra.UserPermissionOverride, 0, len(in))
	for _, o := range in {
		out = append(out, mysqlInfra.UserPermissionOverride{PermissionID: o.PermissionID, Effect: o.Effect})
	}
	return out
}

func parseUserID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid id")
		return 0, false
	}
	return id, true
}
