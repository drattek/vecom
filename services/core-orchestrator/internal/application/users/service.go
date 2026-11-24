// Package users backs the admin "Usuarios" page: listing ecom_api_user,
// creating accounts with a hashed password and managing their access (rol y
// excepciones de permisos, ADR 0009).
//
// Un usuario con role = "admin" tiene acceso total y no usa ecom_roles. El
// resto lleva role = "user" más un role_id opcional y, si hace falta,
// excepciones por usuario (grant/deny) en ecom_user_permissions.
package users

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"core-orchestrator/internal/domain"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

var (
	ErrInvalidUserPayload = errors.New("username and password are required")
	ErrPasswordTooShort   = errors.New("password must be at least 8 characters")
	ErrInvalidRole        = errors.New("role does not exist")
	ErrLastAdmin          = errors.New("cannot remove the last active admin")
	ErrSelfLockout        = errors.New("cannot remove your own admin access or deactivate yourself")
)

// standardUserRole es el valor de ecom_api_user.role para cuentas no admin.
const standardUserRole = "user"

const minPasswordLength = 8

type CreateUserInput struct {
	Username  string
	Password  string
	IsAdmin   bool
	RoleID    *int64
	Overrides []mysqlInfra.UserPermissionOverride
	ActorID   int64
}

type UpdateUserInput struct {
	IsAdmin  bool
	RoleID   *int64
	IsActive bool
	// Overrides nil = no tocar las excepciones; vacío = quitarlas todas.
	Overrides *[]mysqlInfra.UserPermissionOverride
	ActorID   int64
}

// UserDetail es el usuario más sus excepciones de permisos.
type UserDetail struct {
	*mysqlInfra.UserDTO
	Overrides []mysqlInfra.UserPermissionOverride `json:"overrides"`
}

type UserService struct {
	db         *sql.DB
	repository *mysqlInfra.UsersRepository
}

func NewUserService(db *sql.DB, repository *mysqlInfra.UsersRepository) *UserService {
	return &UserService{db: db, repository: repository}
}

func (s *UserService) GetPaginatedUsers(ctx context.Context, offset, pageSize int) (*mysqlInfra.PaginatedUsers, error) {
	if offset < 0 {
		offset = 0
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}

	return s.repository.FindPaginated(ctx, offset, pageSize)
}

func (s *UserService) GetUser(ctx context.Context, id int64) (*UserDetail, error) {
	user, err := s.repository.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	overrides, err := mysqlInfra.NewRBACRepository(s.db).ListUserOverrides(ctx, id)
	if err != nil {
		return nil, err
	}
	return &UserDetail{UserDTO: user, Overrides: overrides}, nil
}

// CreateUser inserta la cuenta y sus excepciones en una sola transacción. The
// plaintext password never reaches the repository layer — it's hashed here and
// discarded.
func (s *UserService) CreateUser(ctx context.Context, input CreateUserInput) (*mysqlInfra.UserDTO, error) {
	username := strings.TrimSpace(input.Username)
	if username == "" || input.Password == "" {
		return nil, ErrInvalidUserPayload
	}
	if len(input.Password) < minPasswordLength {
		return nil, ErrPasswordTooShort
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	role, roleID := resolveRole(input.IsAdmin, input.RoleID)

	var created *mysqlInfra.UserDTO
	err = mysqlInfra.WithinTx(ctx, s.db, func(tx *sql.Tx) error {
		rbac := mysqlInfra.NewRBACRepository(tx)
		if roleID != nil {
			ok, err := rbac.RoleExists(ctx, *roleID)
			if err != nil {
				return err
			}
			if !ok {
				return ErrInvalidRole
			}
		}

		user, err := mysqlInfra.NewUsersRepository(tx).Create(ctx, mysqlInfra.CreateUserInput{
			Username:     username,
			PasswordHash: string(hash),
			Role:         role,
			RoleID:       roleID,
			IsActive:     true,
		})
		if err != nil {
			return err
		}
		created = user

		if !input.IsAdmin && len(input.Overrides) > 0 {
			return rbac.ReplaceUserOverrides(ctx, user.ID, input.Overrides, input.ActorID)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return created, nil
}

// UpdateUser cambia rol, estado y excepciones. Protege contra quedarse sin
// administradores activos y contra que un admin se bloquee a sí mismo.
func (s *UserService) UpdateUser(ctx context.Context, id int64, input UpdateUserInput) (*UserDetail, error) {
	current, err := s.repository.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	wasAdmin := current.Role == domain.AdminRole
	losesAdmin := wasAdmin && (!input.IsAdmin || !input.IsActive)
	if losesAdmin {
		if id == input.ActorID {
			return nil, ErrSelfLockout
		}
		if current.IsActive {
			others, err := s.repository.CountOtherActiveAdmins(ctx, id)
			if err != nil {
				return nil, err
			}
			if others == 0 {
				return nil, ErrLastAdmin
			}
		}
	}
	if id == input.ActorID && !input.IsActive {
		return nil, ErrSelfLockout
	}

	role, roleID := resolveRole(input.IsAdmin, input.RoleID)

	err = mysqlInfra.WithinTx(ctx, s.db, func(tx *sql.Tx) error {
		rbac := mysqlInfra.NewRBACRepository(tx)
		if roleID != nil {
			ok, err := rbac.RoleExists(ctx, *roleID)
			if err != nil {
				return err
			}
			if !ok {
				return ErrInvalidRole
			}
		}
		if err := mysqlInfra.NewUsersRepository(tx).UpdateAccess(ctx, id, role, roleID, input.IsActive); err != nil {
			return err
		}
		if input.IsAdmin {
			// El admin no usa excepciones: se limpian para no dejar basura.
			return rbac.ReplaceUserOverrides(ctx, id, nil, input.ActorID)
		}
		if input.Overrides != nil {
			return rbac.ReplaceUserOverrides(ctx, id, *input.Overrides, input.ActorID)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return s.GetUser(ctx, id)
}

func resolveRole(isAdmin bool, roleID *int64) (string, *int64) {
	if isAdmin {
		return domain.AdminRole, nil
	}
	return standardUserRole, roleID
}
