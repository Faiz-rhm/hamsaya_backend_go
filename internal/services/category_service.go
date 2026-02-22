package services

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/internal/repositories"
	"github.com/hamsaya/backend/internal/utils"
	"go.uber.org/zap"
)

// CategoryService handles business logic for marketplace categories
type CategoryService struct {
	categoryRepo repositories.CategoryRepository
	logger       *zap.Logger
}

// NewCategoryService creates a new category service
func NewCategoryService(categoryRepo repositories.CategoryRepository, logger *zap.Logger) *CategoryService {
	return &CategoryService{
		categoryRepo: categoryRepo,
		logger:       logger,
	}
}

// CreateCategory creates a new marketplace category (admin operation)
func (s *CategoryService) CreateCategory(ctx context.Context, req *models.CreateCategoryRequest) (*models.CategoryResponse, error) {
	// Generate new category ID
	categoryID := uuid.New().String()

	// Set default status if not provided
	status := models.CategoryStatusActive
	if req.Status != "" {
		status = models.CategoryStatus(req.Status)
	}

	category := &models.SellCategory{
		ID:        categoryID,
		Name:      req.Name,
		Icon:      req.Icon,
		Color:     req.Color,
		Status:    status,
		CreatedAt: time.Now(),
	}

	if err := s.categoryRepo.Create(ctx, category); err != nil {
		s.logger.Error("Failed to create category",
			zap.Error(err),
			zap.String("name", req.Name),
		)
		return nil, utils.NewInternalError("Failed to create category", err)
	}

	s.logger.Info("Category created successfully",
		zap.String("category_id", categoryID),
		zap.String("name", req.Name),
	)

	return category.ToCategoryResponse(models.LocaleEN), nil
}

// GetCategory retrieves a category by ID. locale (en, dari, pashto) controls the name in the response.
func (s *CategoryService) GetCategory(ctx context.Context, categoryID string, locale string) (*models.CategoryResponse, error) {
	category, err := s.categoryRepo.GetByID(ctx, categoryID)
	if err != nil {
		s.logger.Error("Failed to get category",
			zap.Error(err),
			zap.String("category_id", categoryID),
		)
		return nil, utils.NewNotFoundError("Category not found", err)
	}

	return category.ToCategoryResponse(locale), nil
}

// GetAllCategories retrieves all categories (admin operation). locale controls the name in the response.
func (s *CategoryService) GetAllCategories(ctx context.Context, locale string) ([]*models.CategoryResponse, error) {
	categories, err := s.categoryRepo.GetAll(ctx)
	if err != nil {
		s.logger.Error("Failed to get all categories", zap.Error(err))
		return nil, utils.NewInternalError("Failed to retrieve categories", err)
	}

	responses := make([]*models.CategoryResponse, len(categories))
	for i, category := range categories {
		responses[i] = category.ToCategoryResponse(locale)
	}

	return responses, nil
}

// GetActiveCategories retrieves only active categories (public operation). locale controls the name in the response.
func (s *CategoryService) GetActiveCategories(ctx context.Context, locale string) ([]*models.CategoryResponse, error) {
	categories, err := s.categoryRepo.GetActiveCategories(ctx)
	if err != nil {
		s.logger.Error("Failed to get active categories", zap.Error(err))
		return nil, utils.NewInternalError("Failed to retrieve categories", err)
	}

	responses := make([]*models.CategoryResponse, len(categories))
	for i, category := range categories {
		responses[i] = category.ToCategoryResponse(locale)
	}

	return responses, nil
}

// UpdateCategory updates an existing category (admin operation)
func (s *CategoryService) UpdateCategory(ctx context.Context, categoryID string, req *models.UpdateCategoryRequest) (*models.CategoryResponse, error) {
	// Get existing category
	category, err := s.categoryRepo.GetByID(ctx, categoryID)
	if err != nil {
		s.logger.Error("Failed to get category for update",
			zap.Error(err),
			zap.String("category_id", categoryID),
		)
		return nil, utils.NewNotFoundError("Category not found", err)
	}

	// Update fields if provided
	if req.Name != nil {
		category.Name = *req.Name
	}
	if req.Icon != nil {
		category.Icon = *req.Icon
	}
	if req.Color != nil {
		category.Color = *req.Color
	}
	if req.Status != nil {
		category.Status = models.CategoryStatus(*req.Status)
	}

	// Save updates
	if err := s.categoryRepo.Update(ctx, category); err != nil {
		s.logger.Error("Failed to update category",
			zap.Error(err),
			zap.String("category_id", categoryID),
		)
		return nil, utils.NewInternalError("Failed to update category", err)
	}

	s.logger.Info("Category updated successfully",
		zap.String("category_id", categoryID),
		zap.String("name", category.Name),
	)

	return category.ToCategoryResponse(models.LocaleEN), nil
}

// DeleteCategory deletes a category (admin operation)
func (s *CategoryService) DeleteCategory(ctx context.Context, categoryID string) error {
	// Check if category exists
	_, err := s.categoryRepo.GetByID(ctx, categoryID)
	if err != nil {
		s.logger.Error("Failed to get category for deletion",
			zap.Error(err),
			zap.String("category_id", categoryID),
		)
		return utils.NewNotFoundError("Category not found", err)
	}

	// Delete the category
	if err := s.categoryRepo.Delete(ctx, categoryID); err != nil {
		s.logger.Error("Failed to delete category",
			zap.Error(err),
			zap.String("category_id", categoryID),
		)
		return utils.NewInternalError("Failed to delete category", err)
	}

	s.logger.Info("Category deleted successfully",
		zap.String("category_id", categoryID),
	)

	return nil
}

// ListCategories retrieves categories with filters. locale controls the name in the response.
func (s *CategoryService) ListCategories(ctx context.Context, filter *models.CategoryListFilter, locale string) ([]*models.CategoryResponse, error) {
	categories, err := s.categoryRepo.List(ctx, filter)
	if err != nil {
		s.logger.Error("Failed to list categories",
			zap.Error(err),
			zap.Any("filter", filter),
		)
		return nil, utils.NewInternalError("Failed to retrieve categories", err)
	}

	responses := make([]*models.CategoryResponse, len(categories))
	for i, category := range categories {
		responses[i] = category.ToCategoryResponse(locale)
	}

	return responses, nil
}

// GetCategoriesByIDs retrieves multiple categories by their IDs. locale controls the name in the response.
func (s *CategoryService) GetCategoriesByIDs(ctx context.Context, categoryIDs []string, locale string) ([]*models.CategoryResponse, error) {
	if len(categoryIDs) == 0 {
		return []*models.CategoryResponse{}, nil
	}

	categories, err := s.categoryRepo.GetByIDs(ctx, categoryIDs)
	if err != nil {
		s.logger.Error("Failed to get categories by IDs",
			zap.Error(err),
			zap.Strings("category_ids", categoryIDs),
		)
		return nil, utils.NewInternalError("Failed to retrieve categories", err)
	}

	responses := make([]*models.CategoryResponse, len(categories))
	for i, category := range categories {
		responses[i] = category.ToCategoryResponse(locale)
	}

	return responses, nil
}
