package http

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"

	"core-orchestrator/internal/application/credentials"
)

type AuthHandler struct {
	service *credentials.AuthService
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func NewAuthHandler(service *credentials.AuthService) *AuthHandler {
	return &AuthHandler{service: service}
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var body loginRequest
	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	body.Username = strings.TrimSpace(body.Username)
	if body.Username == "" || body.Password == "" {
		writeJSONError(w, http.StatusBadRequest, "username and password are required")
		return
	}

	clientIP := clientIPFromRequest(r)
	userAgent := r.UserAgent()

	result, err := h.service.Login(body.Username, body.Password, userAgent, clientIP)
	if err != nil {
		if errors.Is(err, credentials.ErrInvalidCredentials) || errors.Is(err, credentials.ErrUnauthorized) {
			writeJSONError(w, http.StatusUnauthorized, "invalid credentials")
			return
		}

		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

func clientIPFromRequest(r *http.Request) string {
	xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For"))
	if xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}

	xri := strings.TrimSpace(r.Header.Get("X-Real-IP"))
	if xri != "" {
		return xri
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}

	return r.RemoteAddr
}

func writeJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}
