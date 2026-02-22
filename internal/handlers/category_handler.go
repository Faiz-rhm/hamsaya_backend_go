package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/internal/services"
	"github.com/hamsaya/backend/internal/utils"
	"go.uber.org/zap"
)

// categoryLocale returns the requested locale for category names (en, dari, pashto). Default en.
func categoryLocale(c *gin.Context) string {
	if loc := c.Query("locale"); loc != "" {
		loc = strings.ToLower(strings.TrimSpace(loc))
		switch loc {
		case models.LocaleEN, models.LocaleDari, models.LocalePashto:
			return loc
		}
	}
	if lang := c.GetHeader("Accept-Language"); lang != "" {
		// e.g. "ps-AF,ps;q=0.9,en;q=0.8" or "fa-AF"
		for _, part := range strings.Split(lang, ",") {
			part = strings.ToLower(strings.TrimSpace(strings.Split(part, ";")[0]))
			if part == "en" || strings.HasPrefix(part, "en-") {
				return models.LocaleEN
			}
			if part == "fa" || part == "fa-af" || strings.HasPrefix(part, "fa") {
				return models.LocaleDari
			}
			if part == "ps" || part == "pashto" || strings.HasPrefix(part, "ps") {
				return models.LocalePashto
			}
		}
	}
	return models.LocaleEN
}

// CategoryHandler handles HTTP requests for marketplace categories
type CategoryHandler struct {
	categoryService *services.CategoryService
	validator       *utils.Validator
	logger          *zap.Logger
}

// NewCategoryHandler creates a new category handler
func NewCategoryHandler(categoryService *services.CategoryService, validator *utils.Validator, logger *zap.Logger) *CategoryHandler {
	return &CategoryHandler{
		categoryService: categoryService,
		validator:       validator,
		logger:          logger,
	}
}

// CreateCategory handles POST /api/v1/admin/categories
// Admin operation to create a new marketplace category
func (h *CategoryHandler) CreateCategory(c *gin.Context) {
	var req models.CreateCategoryRequest

	// Bind and validate request
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Failed to bind create category request", zap.Error(err))
		utils.SendError(c, http.StatusBadRequest, "Invalid request body", utils.ErrInvalidJSON)
		return
	}

	if err := h.validator.Validate(&req); err != nil {
		h.logger.Error("Validation failed for create category", zap.Error(err))
		utils.SendError(c, http.StatusBadRequest, err.Error(), utils.ErrValidation)
		return
	}

	// Create category
	category, err := h.categoryService.CreateCategory(c.Request.Context(), &req)
	if err != nil {
		h.logger.Error("Failed to create category", zap.Error(err))
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusCreated, "Category created successfully", category)
}

// GetCategory handles GET /api/v1/categories/:category_id
// Retrieves a specific category by ID. Use ?locale=en|dari|pashto or Accept-Language for name.
func (h *CategoryHandler) GetCategory(c *gin.Context) {
	categoryID := c.Param("category_id")

	if categoryID == "" {
		utils.SendError(c, http.StatusBadRequest, "Category ID is required", utils.ErrBadRequest)
		return
	}

	locale := categoryLocale(c)
	category, err := h.categoryService.GetCategory(c.Request.Context(), categoryID, locale)
	if err != nil {
		h.logger.Error("Failed to get category",
			zap.Error(err),
			zap.String("category_id", categoryID),
		)
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Category retrieved successfully", category)
}

// GetAllCategories handles GET /api/v1/admin/categories
// Admin operation to retrieve all categories (including inactive). Use ?locale=en|dari|pashto for name.
func (h *CategoryHandler) GetAllCategories(c *gin.Context) {
	locale := categoryLocale(c)
	categories, err := h.categoryService.GetAllCategories(c.Request.Context(), locale)
	if err != nil {
		h.logger.Error("Failed to get all categories", zap.Error(err))
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Categories retrieved successfully", categories)
}

// ListCategories handles GET /api/v1/categories
// Public endpoint to retrieve active categories for marketplace. Use ?locale=en|dari|pashto or Accept-Language for names.
func (h *CategoryHandler) ListCategories(c *gin.Context) {
	locale := categoryLocale(c)
	categories, err := h.categoryService.GetActiveCategories(c.Request.Context(), locale)
	if err != nil {
		h.logger.Error("Failed to list active categories", zap.Error(err))
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Categories retrieved successfully", categories)
}

// UpdateCategory handles PUT /api/v1/admin/categories/:category_id
// Admin operation to update an existing category
func (h *CategoryHandler) UpdateCategory(c *gin.Context) {
	categoryID := c.Param("category_id")

	if categoryID == "" {
		utils.SendError(c, http.StatusBadRequest, "Category ID is required", utils.ErrBadRequest)
		return
	}

	var req models.UpdateCategoryRequest

	// Bind and validate request
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Failed to bind update category request", zap.Error(err))
		utils.SendError(c, http.StatusBadRequest, "Invalid request body", utils.ErrInvalidJSON)
		return
	}

	if err := h.validator.Validate(&req); err != nil {
		h.logger.Error("Validation failed for update category", zap.Error(err))
		utils.SendError(c, http.StatusBadRequest, err.Error(), utils.ErrValidation)
		return
	}

	// Update category
	category, err := h.categoryService.UpdateCategory(c.Request.Context(), categoryID, &req)
	if err != nil {
		h.logger.Error("Failed to update category",
			zap.Error(err),
			zap.String("category_id", categoryID),
		)
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Category updated successfully", category)
}

// DeleteCategory handles DELETE /api/v1/admin/categories/:category_id
// Admin operation to delete a category
func (h *CategoryHandler) DeleteCategory(c *gin.Context) {
	categoryID := c.Param("category_id")

	if categoryID == "" {
		utils.SendError(c, http.StatusBadRequest, "Category ID is required", utils.ErrBadRequest)
		return
	}

	if err := h.categoryService.DeleteCategory(c.Request.Context(), categoryID); err != nil {
		h.logger.Error("Failed to delete category",
			zap.Error(err),
			zap.String("category_id", categoryID),
		)
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Category deleted successfully", nil)
}

// handleError handles service errors and sends appropriate HTTP responses
func (h *CategoryHandler) handleError(c *gin.Context, err error) {
	// Check if it's an AppError
	if appErr, ok := err.(*utils.AppError); ok {
		utils.SendError(c, appErr.Code, appErr.Message, appErr.Err)
		return
	}

	// Default to internal server error
	h.logger.Error("Unhandled error in category handler", zap.Error(err))
	utils.SendError(c, http.StatusInternalServerError, "An error occurred", err)
}
