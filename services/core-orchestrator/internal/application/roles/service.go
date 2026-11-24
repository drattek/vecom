// Package roles backs Configuración → Roles: CRUD de ecom_roles y la
// asignación de permisos del catálogo (ecom_permissions) a cada rol. Ver ADR
// 0009. "admin" no es un rol de esta tabla: es acceso total fijo.
package roles

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

var (
	ErrInvalidRolePayload = errors.New("role name is required")
)

type RoleInput struct {
	Name          string
	Description   string
	PermissionIDs []int64
	ActorID       int64
}

type RoleService struct {
	db         *sql.DB
	repository *mysqlInfra.RBACRepository
}

func NewRoleService(db *sql.DB, repository *mysqlInfra.RBACRepository) *RoleService {
	return &RoleService{db: db, repository: repository}
}

func (s *RoleService) ListRoles(ctx context.Context) ([]mysqlInfra.RoleDTO, error) {
	return s.repository.ListRoles(ctx)
}

func (s *RoleService) GetRole(ctx context.Context, id int64) (*mysqlInfra.RoleDTO, error) {
	return s.repository.FindRole(ctx, id)
}

func (s *RoleService) ListPermissions(ctx context.Context) ([]mysqlInfra.PermissionDTO, error) {
	return s.repository.ListPermissions(ctx)
}

func (s *RoleService) CreateRole(ctx context.Context, input RoleInput) (*mysqlInfra.RoleDTO, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, ErrInvalidRolePayload
	}
	description := strings.TrimSpace(input.Description)

	var roleID int64
	err := mysqlInfra.WithinTx(ctx, s.db, func(tx *sql.Tx) error {
		repo := mysqlInfra.NewRBACRepository(tx)

		base := slugify(name)
		var id int64
		var err error
		for attempt := 0; attempt < 20; attempt++ {
			code := base
			if attempt > 0 {
				code = fmt.Sprintf("%s_%d", base, attempt+1)
			}
			id, err = repo.CreateRole(ctx, code, name, description, input.ActorID)
			if !errors.Is(err, mysqlInfra.ErrRoleCodeExists) {
				break
			}
		}
		if err != nil {
			return err
		}
		roleID = id
		return repo.ReplaceRolePermissions(ctx, id, input.PermissionIDs)
	})
	if err != nil {
		return nil, err
	}
	return s.repository.FindRole(ctx, roleID)
}

func (s *RoleService) UpdateRole(ctx context.Context, id int64, input RoleInput) (*mysqlInfra.RoleDTO, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, ErrInvalidRolePayload
	}

	err := mysqlInfra.WithinTx(ctx, s.db, func(tx *sql.Tx) error {
		repo := mysqlInfra.NewRBACRepository(tx)
		if err := repo.UpdateRole(ctx, id, name, strings.TrimSpace(input.Description), input.ActorID); err != nil {
			return err
		}
		return repo.ReplaceRolePermissions(ctx, id, input.PermissionIDs)
	})
	if err != nil {
		return nil, err
	}
	return s.repository.FindRole(ctx, id)
}

func (s *RoleService) DeleteRole(ctx context.Context, id int64, actorID int64) error {
	return s.repository.SoftDeleteRole(ctx, id, actorID)
}

// slugify genera el code (UNIQUE) del rol a partir del nombre.
func slugify(name string) string {
	replacer := strings.NewReplacer(
		"á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ü", "u", "ñ", "n",
		"Á", "a", "É", "e", "Í", "i", "Ó", "o", "Ú", "u", "Ü", "u", "Ñ", "n",
	)
	var b strings.Builder
	lastUnderscore := true
	for _, r := range strings.ToLower(replacer.Replace(name)) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastUnderscore = false
		} else if !lastUnderscore {
			b.WriteByte('_')
			lastUnderscore = true
		}
	}
	slug := strings.Trim(b.String(), "_")
	if slug == "" || slug == "admin" {
		slug = "rol_" + slug
	}
	if len(slug) > 40 {
		slug = slug[:40]
	}
	return slug
}
