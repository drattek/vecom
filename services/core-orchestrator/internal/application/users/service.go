// Package users backs the admin "Usuarios" page: listing ecom_api_user and
// creating new accounts with a hashed password. There is no role/permission
// system yet, so every account created here gets role "admin" fixed — see
// the package-level note in the HTTP handler for how that's expected to
// evolve once role-based access control exists.
package users

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"golang.org/x/crypto/bcrypt"

	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

var (
	ErrInvalidUserPayload = errors.New("username and password are required")
	ErrPasswordTooShort   = errors.New("password must be at least 8 characters")
)

// defaultUserRole is fixed for every account created through this service:
// there is no role/permission system yet (see package doc), so "admin" is
// applied uniformly until one exists.
const defaultUserRole = "admin"

const minPasswordLength = 8

type CreateUserInput struct {
	Username string
	Password string
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

// CreateUser is a single statement (INSERT + read-back), no transaction
// needed. The plaintext password never reaches the repository layer — it's
// hashed here and discarded.
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

	return s.repository.Create(ctx, mysqlInfra.CreateUserInput{
		Username:     username,
		PasswordHash: string(hash),
		Role:         defaultUserRole,
		IsActive:     true,
	})
}
