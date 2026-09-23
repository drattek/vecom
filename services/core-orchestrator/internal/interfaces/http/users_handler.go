package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"

	usersApp "core-orchestrator/internal/application/users"
)

// UserHandler serves the admin "Usuarios" page: GET /api/users (paginated
// list) and POST /api/users (create, with password). Unlike most other
// entities in this API, creating a user has no created_by/updated_by to fill
// (ecom_api_user has no audit columns), so this handler doesn't need
// UserFromContext — authentication is already required by the router's
// protected group, same as every other route here.
type UserHandler struct {
	service *usersApp.UserService
}

type createUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
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
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

func (h *UserHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req createUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user, err := h.service.CreateUser(r.Context(), usersApp.CreateUserInput{
		Username: req.Username,
		Password: req.Password,
	})
	if err != nil {
		switch {
		case errors.Is(err, usersApp.ErrInvalidUserPayload):
			writeJSONError(w, http.StatusBadRequest, "username and password are required")
		case errors.Is(err, usersApp.ErrPasswordTooShort):
			writeJSONError(w, http.StatusBadRequest, "password must be at least 8 characters")
		case errors.Is(err, mysqlInfra.ErrUserUsernameAlreadyExists):
			writeJSONError(w, http.StatusConflict, "username already exists")
		default:
			writeJSONError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(user)
}
