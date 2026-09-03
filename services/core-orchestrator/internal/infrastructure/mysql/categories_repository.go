package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var ErrCategoryNotFound = errors.New("category not found")

type CategoryDTO struct {
	ID        int64      `json:"id"`
	Name      string     `json:"name"`
	ParentID  *int64     `json:"parentId,omitempty"`
	CreatedBy int64      `json:"createdBy"`
	UpdatedBy *int64     `json:"updatedBy,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	DeletedAt *time.Time `json:"deletedAt,omitempty"`
}

type CreateCategoryInput struct {
	Name      string
	ParentID  *int64
	CreatedBy int64
}

type UpdateCategoryInput struct {
	Name      string
	ParentID  *int64
	UpdatedBy int64
}

type CategoriesRepository struct {
	db Querier
}

func NewCategoriesRepository(db Querier) *CategoriesRepository {
	return &CategoriesRepository{db: db}
}

// CategoryTreeDTO is a CategoryDTO with its descendants nested under
// Children, so the hierarchy can be rendered directly as a tree.
type CategoryTreeDTO struct {
	ID        int64             `json:"id"`
	Name      string            `json:"name"`
	ParentID  *int64            `json:"parentId,omitempty"`
	CreatedBy int64             `json:"createdBy"`
	UpdatedBy *int64            `json:"updatedBy,omitempty"`
	CreatedAt time.Time         `json:"createdAt"`
	UpdatedAt time.Time         `json:"updatedAt"`
	DeletedAt *time.Time        `json:"deletedAt,omitempty"`
	Children  []CategoryTreeDTO `json:"children"`
}

// FindTree returns every non-deleted category as a nested tree built from
// parent_id: each category's own children are collected under its
// Children key (siblings kept in id ASC order), instead of a flat list.
func (r *CategoriesRepository) FindTree(ctx context.Context) ([]CategoryTreeDTO, error) {
	query := `
		SELECT id, name, parent_id, created_by, updated_by, created_at, updated_at, deleted_at
		FROM ecom_categories
		WHERE deleted_at IS NULL
		ORDER BY id ASC
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("error querying categories: %w", err)
	}
	defer rows.Close()

	categories := make([]CategoryDTO, 0)
	for rows.Next() {
		cat, scanErr := scanCategory(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		categories = append(categories, cat)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating categories: %w", err)
	}

	return buildCategoryTree(categories), nil
}

// buildCategoryTree nests a flat, id-ordered category slice into a tree
// using parent_id. A category whose parent_id points at a row that's
// missing or soft-deleted is treated as a root instead of being dropped.
func buildCategoryTree(categories []CategoryDTO) []CategoryTreeDTO {
	exists := make(map[int64]bool, len(categories))
	for _, cat := range categories {
		exists[cat.ID] = true
	}

	childrenByParent := make(map[int64][]CategoryDTO, len(categories))
	roots := make([]CategoryDTO, 0)
	for _, cat := range categories {
		if cat.ParentID == nil || !exists[*cat.ParentID] {
			roots = append(roots, cat)
			continue
		}
		childrenByParent[*cat.ParentID] = append(childrenByParent[*cat.ParentID], cat)
	}

	var build func(cat CategoryDTO) CategoryTreeDTO
	build = func(cat CategoryDTO) CategoryTreeDTO {
		children := childrenByParent[cat.ID]
		node := CategoryTreeDTO{
			ID:        cat.ID,
			Name:      cat.Name,
			ParentID:  cat.ParentID,
			CreatedBy: cat.CreatedBy,
			UpdatedBy: cat.UpdatedBy,
			CreatedAt: cat.CreatedAt,
			UpdatedAt: cat.UpdatedAt,
			DeletedAt: cat.DeletedAt,
			Children:  make([]CategoryTreeDTO, 0, len(children)),
		}
		for _, child := range children {
			node.Children = append(node.Children, build(child))
		}
		return node
	}

	tree := make([]CategoryTreeDTO, 0, len(roots))
	for _, root := range roots {
		tree = append(tree, build(root))
	}

	return tree
}

func (r *CategoriesRepository) FindByID(ctx context.Context, id int64) (*CategoryDTO, error) {
	query := `
		SELECT id, name, parent_id, created_by, updated_by, created_at, updated_at, deleted_at
		FROM ecom_categories
		WHERE id = ? AND deleted_at IS NULL
		LIMIT 1
	`

	row := r.db.QueryRowContext(ctx, query, id)
	return scanCategoryRow(row)
}

// FindByNameAndParentID looks up a category by name scoped to a parent
// (nil parent means a root category), since name alone isn't unique across
// different branches of the hierarchy.
func (r *CategoriesRepository) FindByNameAndParentID(ctx context.Context, name string, parentID *int64) (*CategoryDTO, error) {
	if parentID == nil {
		query := `
			SELECT id, name, parent_id, created_by, updated_by, created_at, updated_at, deleted_at
			FROM ecom_categories
			WHERE name = ? AND parent_id IS NULL AND deleted_at IS NULL
			ORDER BY id ASC
			LIMIT 1
		`
		return scanCategoryRow(r.db.QueryRowContext(ctx, query, name))
	}

	query := `
		SELECT id, name, parent_id, created_by, updated_by, created_at, updated_at, deleted_at
		FROM ecom_categories
		WHERE name = ? AND parent_id = ? AND deleted_at IS NULL
		ORDER BY id ASC
		LIMIT 1
	`
	return scanCategoryRow(r.db.QueryRowContext(ctx, query, name, *parentID))
}

func (r *CategoriesRepository) FindByParentID(ctx context.Context, parentID int64) ([]CategoryDTO, error) {
	query := `
		SELECT id, name, parent_id, created_by, updated_by, created_at, updated_at, deleted_at
		FROM ecom_categories
		WHERE parent_id = ? AND deleted_at IS NULL
		ORDER BY id ASC
	`

	rows, err := r.db.QueryContext(ctx, query, parentID)
	if err != nil {
		return nil, fmt.Errorf("error querying categories by parent: %w", err)
	}
	defer rows.Close()

	categories := make([]CategoryDTO, 0)
	for rows.Next() {
		cat, scanErr := scanCategory(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		categories = append(categories, cat)
	}

	return categories, rows.Err()
}

func (r *CategoriesRepository) Create(ctx context.Context, input CreateCategoryInput) (*CategoryDTO, error) {
	query := `
		INSERT INTO ecom_categories (name, parent_id, created_by, created_at, updated_at)
		VALUES (?, ?, ?, NOW(), NOW())
	`

	result, err := r.db.ExecContext(ctx, query, input.Name, input.ParentID, input.CreatedBy)
	if err != nil {
		return nil, fmt.Errorf("error creating category: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("error getting last insert id: %w", err)
	}

	return r.FindByID(ctx, id)
}

func (r *CategoriesRepository) Update(ctx context.Context, id int64, input UpdateCategoryInput) (*CategoryDTO, error) {
	query := `
		UPDATE ecom_categories
		SET name = ?, parent_id = ?, updated_by = ?, updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`

	result, err := r.db.ExecContext(ctx, query, input.Name, input.ParentID, input.UpdatedBy, id)
	if err != nil {
		return nil, fmt.Errorf("error updating category: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("error getting rows affected: %w", err)
	}

	if affected == 0 {
		return nil, ErrCategoryNotFound
	}

	return r.FindByID(ctx, id)
}

func (r *CategoriesRepository) SoftDelete(ctx context.Context, id int64) error {
	query := `
		UPDATE ecom_categories
		SET deleted_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("error deleting category: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error getting rows affected: %w", err)
	}

	if affected == 0 {
		return ErrCategoryNotFound
	}

	return nil
}

func scanCategory(rows *sql.Rows) (CategoryDTO, error) {
	var cat CategoryDTO
	err := rows.Scan(
		&cat.ID,
		&cat.Name,
		&cat.ParentID,
		&cat.CreatedBy,
		&cat.UpdatedBy,
		&cat.CreatedAt,
		&cat.UpdatedAt,
		&cat.DeletedAt,
	)
	if err != nil {
		return CategoryDTO{}, fmt.Errorf("error scanning category: %w", err)
	}
	return cat, nil
}

func scanCategoryRow(row *sql.Row) (*CategoryDTO, error) {
	var cat CategoryDTO
	err := row.Scan(
		&cat.ID,
		&cat.Name,
		&cat.ParentID,
		&cat.CreatedBy,
		&cat.UpdatedBy,
		&cat.CreatedAt,
		&cat.UpdatedAt,
		&cat.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrCategoryNotFound
		}
		return nil, fmt.Errorf("error scanning category: %w", err)
	}
	return &cat, nil
}
