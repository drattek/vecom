package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var ErrUserNotFound = errors.New("user not found")
var ErrUserUsernameAlreadyExists = errors.New("username already exists")

// UserDTO is the read shape for ecom_api_user — password_hash is never
// exposed here or in any other DTO this repository returns.
type UserDTO struct {
	ID        int64     `json:"id"`
	Username  string    `json:"username"`
	Role      string    `json:"role"`
	IsActive  bool      `json:"isActive"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type PaginatedUsers struct {
	Total    int64     `json:"total"`
	Offset   int       `json:"offset"`
	PageSize int       `json:"pageSize"`
	Users    []UserDTO `json:"users"`
}

// CreateUserInput carries an already-hashed password — hashing happens in
// the application layer (users.Service), never here.
type CreateUserInput struct {
	Username     string
	PasswordHash string
	Role         string
	IsActive     bool
}

type UsersRepository struct {
	db Querier
}

func NewUsersRepository(db Querier) *UsersRepository {
	return &UsersRepository{db: db}
}

func (r *UsersRepository) FindPaginated(ctx context.Context, offset, pageSize int) (*PaginatedUsers, error) {
	var total int64
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM ecom_api_user").Scan(&total); err != nil {
		return nil, fmt.Errorf("error counting users: %w", err)
	}

	query := `
		SELECT id, username, role, is_active, created_at, updated_at
		FROM ecom_api_user
		ORDER BY id ASC
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.QueryContext(ctx, query, pageSize, offset)
	if err != nil {
		return nil, fmt.Errorf("error querying users: %w", err)
	}
	defer rows.Close()

	users := make([]UserDTO, 0)
	for rows.Next() {
		user, scanErr := scanUser(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating users: %w", err)
	}

	return &PaginatedUsers{
		Total:    total,
		Offset:   offset,
		PageSize: pageSize,
		Users:    users,
	}, nil
}

func (r *UsersRepository) FindByID(ctx context.Context, id int64) (*UserDTO, error) {
	query := `
		SELECT id, username, role, is_active, created_at, updated_at
		FROM ecom_api_user
		WHERE id = ?
		LIMIT 1
	`

	row := r.db.QueryRowContext(ctx, query, id)
	return scanUserRow(row)
}

func (r *UsersRepository) Create(ctx context.Context, input CreateUserInput) (*UserDTO, error) {
	query := `
		INSERT INTO ecom_api_user (username, password_hash, role, is_active)
		VALUES (?, ?, ?, ?)
	`

	result, err := r.db.ExecContext(ctx, query, input.Username, input.PasswordHash, input.Role, input.IsActive)
	if err != nil {
		if isDuplicateKeyError(err) {
			return nil, ErrUserUsernameAlreadyExists
		}
		return nil, fmt.Errorf("error creating user: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("error getting user id: %w", err)
	}

	return r.FindByID(ctx, id)
}

func scanUser(rows *sql.Rows) (UserDTO, error) {
	var user UserDTO
	if err := rows.Scan(&user.ID, &user.Username, &user.Role, &user.IsActive, &user.CreatedAt, &user.UpdatedAt); err != nil {
		return UserDTO{}, fmt.Errorf("error scanning user: %w", err)
	}
	return user, nil
}

func scanUserRow(row *sql.Row) (*UserDTO, error) {
	var user UserDTO
	err := row.Scan(&user.ID, &user.Username, &user.Role, &user.IsActive, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("error scanning user: %w", err)
	}
	return &user, nil
}
