package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

var (
	ErrRoleNotFound      = errors.New("role not found")
	ErrRoleCodeExists    = errors.New("role code already exists")
	ErrRoleHasUsers      = errors.New("role has users assigned")
	ErrPermissionInvalid = errors.New("permission id does not exist")
)

type PermissionDTO struct {
	ID          int64  `json:"id"`
	Resource    string `json:"resource"`
	Action      string `json:"action"`
	Description string `json:"description"`
}

type RoleDTO struct {
	ID              int64   `json:"id"`
	Code            string  `json:"code"`
	Name            string  `json:"name"`
	Description     string  `json:"description"`
	PermissionCount int     `json:"permissionCount"`
	UserCount       int     `json:"userCount"`
	PermissionIDs   []int64 `json:"permissionIds"`
}

type UserPermissionOverride struct {
	PermissionID int64  `json:"permissionId"`
	Effect       string `json:"effect"` // "grant" | "deny"
}

// RBACRepository agrupa las tablas ecom_roles, ecom_permissions,
// ecom_role_permissions y ecom_user_permissions.
type RBACRepository struct {
	db Querier
}

func NewRBACRepository(db Querier) *RBACRepository {
	return &RBACRepository{db: db}
}

// EffectivePermissions = (permisos del rol ∪ grants del usuario) − denies del
// usuario, como "recurso.accion". Un rol dado de baja (deleted_at) no aporta.
func (r *RBACRepository) EffectivePermissions(ctx context.Context, userID int64, roleID *int64) ([]string, error) {
	var roleArg any
	if roleID != nil {
		roleArg = *roleID
	}

	const query = `
		SELECT CONCAT(p.resource, '.', p.action)
		FROM ecom_permissions p
		WHERE (
			p.id IN (
				SELECT rp.permission_id
				FROM ecom_role_permissions rp
				INNER JOIN ecom_roles ro ON ro.id = rp.role_id AND ro.deleted_at IS NULL
				WHERE rp.role_id = ?
			)
			OR p.id IN (
				SELECT up.permission_id FROM ecom_user_permissions up
				WHERE up.user_id = ? AND up.effect = 'grant'
			)
		)
		AND p.id NOT IN (
			SELECT up.permission_id FROM ecom_user_permissions up
			WHERE up.user_id = ? AND up.effect = 'deny'
		)
		ORDER BY p.resource, p.action
	`

	rows, err := r.db.QueryContext(ctx, query, roleArg, userID, userID)
	if err != nil {
		return nil, fmt.Errorf("error querying effective permissions: %w", err)
	}
	defer rows.Close()

	perms := make([]string, 0)
	for rows.Next() {
		var perm string
		if err := rows.Scan(&perm); err != nil {
			return nil, fmt.Errorf("error scanning permission: %w", err)
		}
		perms = append(perms, perm)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating permissions: %w", err)
	}
	return perms, nil
}

func (r *RBACRepository) ListPermissions(ctx context.Context) ([]PermissionDTO, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, resource, action, COALESCE(description, '')
		FROM ecom_permissions
		ORDER BY resource, FIELD(action, 'view', 'create', 'edit', 'delete'), action
	`)
	if err != nil {
		return nil, fmt.Errorf("error listing permissions: %w", err)
	}
	defer rows.Close()

	out := make([]PermissionDTO, 0)
	for rows.Next() {
		var p PermissionDTO
		if err := rows.Scan(&p.ID, &p.Resource, &p.Action, &p.Description); err != nil {
			return nil, fmt.Errorf("error scanning permission: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *RBACRepository) ListRoles(ctx context.Context) ([]RoleDTO, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT ro.id, ro.code, ro.name, COALESCE(ro.description, ''),
		       (SELECT COUNT(*) FROM ecom_role_permissions rp WHERE rp.role_id = ro.id),
		       (SELECT COUNT(*) FROM ecom_api_user u WHERE u.role_id = ro.id)
		FROM ecom_roles ro
		WHERE ro.deleted_at IS NULL
		ORDER BY ro.name
	`)
	if err != nil {
		return nil, fmt.Errorf("error listing roles: %w", err)
	}
	defer rows.Close()

	out := make([]RoleDTO, 0)
	for rows.Next() {
		var ro RoleDTO
		if err := rows.Scan(&ro.ID, &ro.Code, &ro.Name, &ro.Description, &ro.PermissionCount, &ro.UserCount); err != nil {
			return nil, fmt.Errorf("error scanning role: %w", err)
		}
		ro.PermissionIDs = []int64{}
		out = append(out, ro)
	}
	return out, rows.Err()
}

func (r *RBACRepository) FindRole(ctx context.Context, id int64) (*RoleDTO, error) {
	var ro RoleDTO
	err := r.db.QueryRowContext(ctx, `
		SELECT ro.id, ro.code, ro.name, COALESCE(ro.description, ''),
		       (SELECT COUNT(*) FROM ecom_role_permissions rp WHERE rp.role_id = ro.id),
		       (SELECT COUNT(*) FROM ecom_api_user u WHERE u.role_id = ro.id)
		FROM ecom_roles ro
		WHERE ro.id = ? AND ro.deleted_at IS NULL
	`, id).Scan(&ro.ID, &ro.Code, &ro.Name, &ro.Description, &ro.PermissionCount, &ro.UserCount)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrRoleNotFound
		}
		return nil, fmt.Errorf("error finding role: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, `SELECT permission_id FROM ecom_role_permissions WHERE role_id = ? ORDER BY permission_id`, id)
	if err != nil {
		return nil, fmt.Errorf("error listing role permissions: %w", err)
	}
	defer rows.Close()

	ro.PermissionIDs = []int64{}
	for rows.Next() {
		var pid int64
		if err := rows.Scan(&pid); err != nil {
			return nil, fmt.Errorf("error scanning role permission: %w", err)
		}
		ro.PermissionIDs = append(ro.PermissionIDs, pid)
	}
	return &ro, rows.Err()
}

func (r *RBACRepository) CreateRole(ctx context.Context, code, name, description string, createdBy int64) (int64, error) {
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO ecom_roles (code, name, description, created_by, updated_by)
		VALUES (?, ?, NULLIF(?, ''), ?, ?)
	`, code, name, description, createdBy, createdBy)
	if err != nil {
		if isDuplicateKeyError(err) {
			return 0, ErrRoleCodeExists
		}
		return 0, fmt.Errorf("error creating role: %w", err)
	}
	return res.LastInsertId()
}

func (r *RBACRepository) UpdateRole(ctx context.Context, id int64, name, description string, updatedBy int64) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE ecom_roles SET name = ?, description = NULLIF(?, ''), updated_by = ?
		WHERE id = ? AND deleted_at IS NULL
	`, name, description, updatedBy, id)
	if err != nil {
		return fmt.Errorf("error updating role: %w", err)
	}
	// MySQL devuelve 0 filas afectadas si no cambió nada: se confirma existencia aparte.
	if n, _ := res.RowsAffected(); n == 0 {
		if _, err := r.FindRole(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

// SoftDeleteRole rechaza la baja si aún hay usuarios con el rol (no se deja a
// nadie sin permisos por accidente).
func (r *RBACRepository) SoftDeleteRole(ctx context.Context, id int64, updatedBy int64) error {
	var users int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM ecom_api_user WHERE role_id = ?`, id).Scan(&users); err != nil {
		return fmt.Errorf("error counting role users: %w", err)
	}
	if users > 0 {
		return ErrRoleHasUsers
	}

	res, err := r.db.ExecContext(ctx, `
		UPDATE ecom_roles
		SET deleted_at = CURRENT_TIMESTAMP, updated_by = ?, code = CONCAT(code, '#', id)
		WHERE id = ? AND deleted_at IS NULL
	`, updatedBy, id)
	if err != nil {
		return fmt.Errorf("error deleting role: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrRoleNotFound
	}
	return nil
}

// ReplaceRolePermissions deja el rol exactamente con los permisos dados.
// Debe llamarse dentro de una transacción (varias sentencias).
func (r *RBACRepository) ReplaceRolePermissions(ctx context.Context, roleID int64, permissionIDs []int64) error {
	if _, err := r.db.ExecContext(ctx, `DELETE FROM ecom_role_permissions WHERE role_id = ?`, roleID); err != nil {
		return fmt.Errorf("error clearing role permissions: %w", err)
	}
	for _, pid := range uniqueIDs(permissionIDs) {
		if _, err := r.db.ExecContext(ctx, `INSERT INTO ecom_role_permissions (role_id, permission_id) VALUES (?, ?)`, roleID, pid); err != nil {
			if isForeignKeyConstraintError(err) {
				return ErrPermissionInvalid
			}
			return fmt.Errorf("error inserting role permission: %w", err)
		}
	}
	return nil
}

func (r *RBACRepository) ListUserOverrides(ctx context.Context, userID int64) ([]UserPermissionOverride, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT permission_id, effect FROM ecom_user_permissions WHERE user_id = ? ORDER BY permission_id`, userID)
	if err != nil {
		return nil, fmt.Errorf("error listing user overrides: %w", err)
	}
	defer rows.Close()

	out := make([]UserPermissionOverride, 0)
	for rows.Next() {
		var o UserPermissionOverride
		if err := rows.Scan(&o.PermissionID, &o.Effect); err != nil {
			return nil, fmt.Errorf("error scanning user override: %w", err)
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// ReplaceUserOverrides reemplaza todas las excepciones del usuario. Dentro de
// una transacción.
func (r *RBACRepository) ReplaceUserOverrides(ctx context.Context, userID int64, overrides []UserPermissionOverride, createdBy int64) error {
	if _, err := r.db.ExecContext(ctx, `DELETE FROM ecom_user_permissions WHERE user_id = ?`, userID); err != nil {
		return fmt.Errorf("error clearing user overrides: %w", err)
	}
	seen := make(map[int64]bool)
	for _, o := range overrides {
		if seen[o.PermissionID] {
			continue
		}
		seen[o.PermissionID] = true
		effect := strings.ToLower(o.Effect)
		if effect != "grant" && effect != "deny" {
			return fmt.Errorf("invalid effect %q", o.Effect)
		}
		if _, err := r.db.ExecContext(ctx, `
			INSERT INTO ecom_user_permissions (user_id, permission_id, effect, created_by) VALUES (?, ?, ?, ?)
		`, userID, o.PermissionID, effect, createdBy); err != nil {
			if isForeignKeyConstraintError(err) {
				return ErrPermissionInvalid
			}
			return fmt.Errorf("error inserting user override: %w", err)
		}
	}
	return nil
}

func (r *RBACRepository) RoleExists(ctx context.Context, id int64) (bool, error) {
	var n int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM ecom_roles WHERE id = ? AND deleted_at IS NULL`, id).Scan(&n); err != nil {
		return false, fmt.Errorf("error checking role: %w", err)
	}
	return n > 0, nil
}

func uniqueIDs(ids []int64) []int64 {
	seen := make(map[int64]bool, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}
