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
	RoleID    *int64    `json:"roleId"`
	RoleName  string    `json:"roleName"`
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
	RoleID       *int64
	IsActive     bool
}

// userSelect: la lista y el detalle comparten columnas (rol resuelto a nombre).
const userSelect = `
	SELECT u.id, u.username, u.role, u.role_id, COALESCE(ro.name, ''), u.is_active, u.created_at, u.updated_at
	FROM ecom_api_user u
	LEFT JOIN ecom_roles ro ON ro.id = u.role_id AND ro.deleted_at IS NULL
`

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

	query := userSelect + `
		ORDER BY u.id ASC
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
	query := userSelect + `
		WHERE u.id = ?
		LIMIT 1
	`

	row := r.db.QueryRowContext(ctx, query, id)
	return scanUserRow(row)
}

func (r *UsersRepository) Create(ctx context.Context, input CreateUserInput) (*UserDTO, error) {
	query := `
		INSERT INTO ecom_api_user (username, password_hash, role, role_id, is_active)
		VALUES (?, ?, ?, ?, ?)
	`

	result, err := r.db.ExecContext(ctx, query, input.Username, input.PasswordHash, input.Role, input.RoleID, input.IsActive)
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

// UpdateAccess cambia rol (texto + role_id) y estado de una cuenta.
func (r *UsersRepository) UpdateAccess(ctx context.Context, id int64, role string, roleID *int64, isActive bool) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE ecom_api_user SET role = ?, role_id = ?, is_active = ? WHERE id = ?
	`, role, roleID, isActive, id)
	if err != nil {
		return fmt.Errorf("error updating user: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		if _, err := r.FindByID(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

// CountOtherActiveAdmins evita dejar el sistema sin ningún admin activo.
func (r *UsersRepository) CountOtherActiveAdmins(ctx context.Context, excludeID int64) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM ecom_api_user WHERE role = 'admin' AND is_active = 1 AND id <> ?
	`, excludeID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("error counting admins: %w", err)
	}
	return n, nil
}

func scanUser(rows *sql.Rows) (UserDTO, error) {
	var user UserDTO
	if err := rows.Scan(&user.ID, &user.Username, &user.Role, &user.RoleID, &user.RoleName, &user.IsActive, &user.CreatedAt, &user.UpdatedAt); err != nil {
		return UserDTO{}, fmt.Errorf("error scanning user: %w", err)
	}
	return user, nil
}

func scanUserRow(row *sql.Row) (*UserDTO, error) {
	var user UserDTO
	err := row.Scan(&user.ID, &user.Username, &user.Role, &user.RoleID, &user.RoleName, &user.IsActive, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("error scanning user: %w", err)
	}
	return &user, nil
}
