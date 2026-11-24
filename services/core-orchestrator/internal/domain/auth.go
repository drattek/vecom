package domain

import "time"

// AdminRole es el valor de ecom_api_user.role con acceso total: un usuario
// admin nunca pasa por ecom_roles / ecom_permissions (ver ADR 0009).
const AdminRole = "admin"

type AuthUser struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	IsActive bool   `json:"isActive"`
	// RoleID / RoleName: rol asignado en ecom_roles (nil para admin y para
	// cuentas técnicas sin rol).
	RoleID   *int64 `json:"roleId"`
	RoleName string `json:"roleName,omitempty"`
	// Permissions son los permisos efectivos "recurso.accion" (rol ∪ grants −
	// denies). Vacío para admin: tiene acceso total y no se calcula.
	Permissions []string `json:"permissions"`
}

// IsAdmin indica acceso total sin consultar permisos.
func (u *AuthUser) IsAdmin() bool {
	return u != nil && u.Role == AdminRole
}

// HasAnyPermission: admin siempre pasa; el resto necesita al menos uno.
func (u *AuthUser) HasAnyPermission(required ...string) bool {
	if u == nil {
		return false
	}
	if u.IsAdmin() {
		return true
	}
	for _, need := range required {
		for _, have := range u.Permissions {
			if have == need {
				return true
			}
		}
	}
	return false
}

type LoginResult struct {
	AccessToken string    `json:"accessToken"`
	TokenType   string    `json:"tokenType"`
	ExpiresAt   time.Time `json:"expiresAt"`
	User        AuthUser  `json:"user"`
}
