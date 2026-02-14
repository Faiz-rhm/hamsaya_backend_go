package repositories

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/pkg/database"
	"github.com/jackc/pgx/v5"
)

// CategoryRepository defines the interface for category operations
type CategoryRepository interface {
	// Basic CRUD operations
	Create(ctx context.Context, category *models.SellCategory) error
	GetByID(ctx context.Context, categoryID string) (*models.SellCategory, error)
	GetAll(ctx context.Context) ([]*models.SellCategory, error)
	Update(ctx context.Context, category *models.SellCategory) error
	Delete(ctx context.Context, categoryID string) error

	// Advanced operations
	List(ctx context.Context, filter *models.CategoryListFilter) ([]*models.SellCategory, error)
	GetByIDs(ctx context.Context, categoryIDs []string) ([]*models.SellCategory, error)
	GetActiveCategories(ctx context.Context) ([]*models.SellCategory, error)
}

type categoryRepository struct {
	db *database.DB
}

// NewCategoryRepository creates a new category repository
func NewCategoryRepository(db *database.DB) CategoryRepository {
	return &categoryRepository{db: db}
}

// Create creates a new category
func (r *categoryRepository) Create(ctx context.Context, category *models.SellCategory) error {
	iconJSON, err := json.Marshal(category.Icon)
	if err != nil {
		return fmt.Errorf("failed to marshal icon: %w", err)
	}

	query := `
		INSERT INTO sell_categories (
			id, name, icon, color, status, created_at
		) VALUES ($1, $2, $3, $4, $5, $6)
	`

	_, err = r.db.Pool.Exec(ctx, query,
		category.ID,
		category.Name,
		iconJSON,
		category.Color,
		category.Status,
		category.CreatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create category: %w", err)
	}

	return nil
}

// GetByID retrieves a category by its ID
func (r *categoryRepository) GetByID(ctx context.Context, categoryID string) (*models.SellCategory, error) {
	query := `
		SELECT id, name, icon, color, status, created_at
		FROM sell_categories
		WHERE id = $1
	`

	var category models.SellCategory
	var iconJSON []byte

	err := r.db.Pool.QueryRow(ctx, query, categoryID).Scan(
		&category.ID,
		&category.Name,
		&iconJSON,
		&category.Color,
		&category.Status,
		&category.CreatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("category not found")
		}
		return nil, fmt.Errorf("failed to get category: %w", err)
	}

	if err := json.Unmarshal(iconJSON, &category.Icon); err != nil {
		return nil, fmt.Errorf("failed to unmarshal icon: %w", err)
	}

	return &category, nil
}

// GetAll retrieves all categories
func (r *categoryRepository) GetAll(ctx context.Context) ([]*models.SellCategory, error) {
	query := `
		SELECT id, name, icon, color, status, created_at
		FROM sell_categories
		ORDER BY name ASC
	`

	rows, err := r.db.Pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get categories: %w", err)
	}
	defer rows.Close()

	var categories []*models.SellCategory

	for rows.Next() {
		var category models.SellCategory
		var iconJSON []byte

		if err := rows.Scan(
			&category.ID,
			&category.Name,
			&iconJSON,
			&category.Color,
			&category.Status,
			&category.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan category: %w", err)
		}

		if err := json.Unmarshal(iconJSON, &category.Icon); err != nil {
			return nil, fmt.Errorf("failed to unmarshal icon: %w", err)
		}

		categories = append(categories, &category)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating categories: %w", err)
	}

	return categories, nil
}

// Update updates an existing category
func (r *categoryRepository) Update(ctx context.Context, category *models.SellCategory) error {
	iconJSON, err := json.Marshal(category.Icon)
	if err != nil {
		return fmt.Errorf("failed to marshal icon: %w", err)
	}

	query := `
		UPDATE sell_categories
		SET name = $1, icon = $2, color = $3, status = $4
		WHERE id = $5
	`

	result, err := r.db.Pool.Exec(ctx, query,
		category.Name,
		iconJSON,
		category.Color,
		category.Status,
		category.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update category: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("category not found")
	}

	return nil
}

// Delete deletes a category by ID
func (r *categoryRepository) Delete(ctx context.Context, categoryID string) error {
	query := `DELETE FROM sell_categories WHERE id = $1`

	result, err := r.db.Pool.Exec(ctx, query, categoryID)
	if err != nil {
		return fmt.Errorf("failed to delete category: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("category not found")
	}

	return nil
}

// List retrieves categories with optional filters
func (r *categoryRepository) List(ctx context.Context, filter *models.CategoryListFilter) ([]*models.SellCategory, error) {
	query := `
		SELECT id, name, icon, color, status, created_at
		FROM sell_categories
	`

	var conditions []string
	var args []interface{}
	argCount := 1

	// Apply status filter
	if filter.Status != nil {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argCount))
		args = append(args, *filter.Status)
		argCount++
	}

	// Add WHERE clause if there are conditions
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	// Add ordering
	query += " ORDER BY name ASC"

	// Add pagination
	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argCount)
		args = append(args, filter.Limit)
		argCount++
	}

	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argCount)
		args = append(args, filter.Offset)
	}

	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list categories: %w", err)
	}
	defer rows.Close()

	var categories []*models.SellCategory

	for rows.Next() {
		var category models.SellCategory
		var iconJSON []byte

		if err := rows.Scan(
			&category.ID,
			&category.Name,
			&iconJSON,
			&category.Color,
			&category.Status,
			&category.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan category: %w", err)
		}

		if err := json.Unmarshal(iconJSON, &category.Icon); err != nil {
			return nil, fmt.Errorf("failed to unmarshal icon: %w", err)
		}

		categories = append(categories, &category)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating categories: %w", err)
	}

	return categories, nil
}

// GetByIDs retrieves multiple categories by their IDs
func (r *categoryRepository) GetByIDs(ctx context.Context, categoryIDs []string) ([]*models.SellCategory, error) {
	if len(categoryIDs) == 0 {
		return []*models.SellCategory{}, nil
	}

	query := `
		SELECT id, name, icon, color, status, created_at
		FROM sell_categories
		WHERE id = ANY($1)
		ORDER BY name ASC
	`

	rows, err := r.db.Pool.Query(ctx, query, categoryIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get categories by IDs: %w", err)
	}
	defer rows.Close()

	var categories []*models.SellCategory

	for rows.Next() {
		var category models.SellCategory
		var iconJSON []byte

		if err := rows.Scan(
			&category.ID,
			&category.Name,
			&iconJSON,
			&category.Color,
			&category.Status,
			&category.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan category: %w", err)
		}

		if err := json.Unmarshal(iconJSON, &category.Icon); err != nil {
			return nil, fmt.Errorf("failed to unmarshal icon: %w", err)
		}

		categories = append(categories, &category)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating categories: %w", err)
	}

	return categories, nil
}

// GetActiveCategories retrieves all active categories
func (r *categoryRepository) GetActiveCategories(ctx context.Context) ([]*models.SellCategory, error) {
	status := models.CategoryStatusActive
	filter := &models.CategoryListFilter{
		Status: &status,
		Limit:  0,
		Offset: 0,
	}

	return r.List(ctx, filter)
}
