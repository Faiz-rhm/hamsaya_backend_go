package services

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/hamsaya/backend/internal/mocks"
	"github.com/hamsaya/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
)

// newTestCategoryService creates a CategoryService wired to the given mock.
func newTestCategoryService(categoryRepo *mocks.MockCategoryRepository) *CategoryService {
	return NewCategoryService(categoryRepo, zap.NewNop())
}

// testSellCategory is a small inline factory used across category tests.
func testSellCategory(id, name string) *models.SellCategory {
	return &models.SellCategory{
		ID:        id,
		Name:      name,
		Icon:      models.CategoryIcon{Name: "tag", Library: "material"},
		Color:     "#FFFFFF",
		Status:    models.CategoryStatusActive,
		CreatedAt: time.Now(),
	}
}

// ---------------------------------------------------------------------------
// TestCategoryService_CreateCategory
// ---------------------------------------------------------------------------

func TestCategoryService_CreateCategory(t *testing.T) {
	tests := []struct {
		name          string
		req           *models.CreateCategoryRequest
		setupMocks    func(*mocks.MockCategoryRepository)
		expectError   bool
		expectedError string
	}{
		{
			name: "success",
			req: &models.CreateCategoryRequest{
				Name:  "Electronics",
				Icon:  models.CategoryIcon{Name: "computer", Library: "material"},
				Color: "#0000FF",
			},
			setupMocks: func(cr *mocks.MockCategoryRepository) {
				cr.On("Create", mock.Anything, mock.AnythingOfType("*models.SellCategory")).Return(nil)
			},
			expectError: false,
		},
		{
			name: "repo failure",
			req: &models.CreateCategoryRequest{
				Name:  "Books",
				Icon:  models.CategoryIcon{Name: "book", Library: "material"},
				Color: "#FF0000",
			},
			setupMocks: func(cr *mocks.MockCategoryRepository) {
				cr.On("Create", mock.Anything, mock.AnythingOfType("*models.SellCategory")).Return(errors.New("db error"))
			},
			expectError:   true,
			expectedError: "db error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			categoryRepo := new(mocks.MockCategoryRepository)
			tt.setupMocks(categoryRepo)

			svc := newTestCategoryService(categoryRepo)
			resp, err := svc.CreateCategory(context.Background(), tt.req)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, resp)
				assert.Contains(t, strings.ToLower(err.Error()), strings.ToLower(tt.expectedError))
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.Equal(t, tt.req.Name, resp.Name)
			}

			categoryRepo.AssertExpectations(t)
		})
	}
}

// ---------------------------------------------------------------------------
// TestCategoryService_GetCategory
// ---------------------------------------------------------------------------

func TestCategoryService_GetCategory(t *testing.T) {
	tests := []struct {
		name          string
		categoryID    string
		locale        string
		setupMocks    func(*mocks.MockCategoryRepository)
		expectError   bool
		expectedError string
		expectedName  string
	}{
		{
			name:       "not found",
			categoryID: "cat-999",
			locale:     models.LocaleEN,
			setupMocks: func(cr *mocks.MockCategoryRepository) {
				cr.On("GetByID", mock.Anything, "cat-999").Return(nil, errors.New("record not found"))
			},
			expectError:   true,
			expectedError: "not found",
		},
		{
			name:       "success",
			categoryID: "cat-1",
			locale:     models.LocaleEN,
			setupMocks: func(cr *mocks.MockCategoryRepository) {
				cr.On("GetByID", mock.Anything, "cat-1").Return(testSellCategory("cat-1", "Electronics"), nil)
			},
			expectError:  false,
			expectedName: "Electronics",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			categoryRepo := new(mocks.MockCategoryRepository)
			tt.setupMocks(categoryRepo)

			svc := newTestCategoryService(categoryRepo)
			resp, err := svc.GetCategory(context.Background(), tt.categoryID, tt.locale)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, resp)
				assert.Contains(t, strings.ToLower(err.Error()), strings.ToLower(tt.expectedError))
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.Equal(t, tt.expectedName, resp.Name)
			}

			categoryRepo.AssertExpectations(t)
		})
	}
}

// ---------------------------------------------------------------------------
// TestCategoryService_GetAllCategories
// ---------------------------------------------------------------------------

func TestCategoryService_GetAllCategories(t *testing.T) {
	tests := []struct {
		name          string
		locale        string
		setupMocks    func(*mocks.MockCategoryRepository)
		expectError   bool
		expectedCount int
	}{
		{
			name:   "success",
			locale: models.LocaleEN,
			setupMocks: func(cr *mocks.MockCategoryRepository) {
				categories := []*models.SellCategory{
					testSellCategory("cat-1", "Electronics"),
					testSellCategory("cat-2", "Clothing"),
					testSellCategory("cat-3", "Books"),
				}
				cr.On("GetAll", mock.Anything).Return(categories, nil)
			},
			expectError:   false,
			expectedCount: 3,
		},
		{
			name:   "empty",
			locale: models.LocaleEN,
			setupMocks: func(cr *mocks.MockCategoryRepository) {
				cr.On("GetAll", mock.Anything).Return([]*models.SellCategory{}, nil)
			},
			expectError:   false,
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			categoryRepo := new(mocks.MockCategoryRepository)
			tt.setupMocks(categoryRepo)

			svc := newTestCategoryService(categoryRepo)
			resp, err := svc.GetAllCategories(context.Background(), tt.locale)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, resp, tt.expectedCount)
			}

			categoryRepo.AssertExpectations(t)
		})
	}
}

// ---------------------------------------------------------------------------
// TestCategoryService_GetActiveCategories
// ---------------------------------------------------------------------------

func TestCategoryService_GetActiveCategories(t *testing.T) {
	tests := []struct {
		name          string
		locale        string
		setupMocks    func(*mocks.MockCategoryRepository)
		expectError   bool
		expectedCount int
	}{
		{
			name:   "success",
			locale: models.LocaleEN,
			setupMocks: func(cr *mocks.MockCategoryRepository) {
				categories := []*models.SellCategory{
					testSellCategory("cat-1", "Electronics"),
					testSellCategory("cat-2", "Clothing"),
				}
				cr.On("GetActiveCategories", mock.Anything).Return(categories, nil)
			},
			expectError:   false,
			expectedCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			categoryRepo := new(mocks.MockCategoryRepository)
			tt.setupMocks(categoryRepo)

			svc := newTestCategoryService(categoryRepo)
			resp, err := svc.GetActiveCategories(context.Background(), tt.locale)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, resp, tt.expectedCount)
			}

			categoryRepo.AssertExpectations(t)
		})
	}
}

// ---------------------------------------------------------------------------
// TestCategoryService_UpdateCategory
// ---------------------------------------------------------------------------

func TestCategoryService_UpdateCategory(t *testing.T) {
	updatedName := "Updated Electronics"
	updatedColor := "#123456"

	tests := []struct {
		name          string
		categoryID    string
		req           *models.UpdateCategoryRequest
		setupMocks    func(*mocks.MockCategoryRepository)
		expectError   bool
		expectedError string
		expectedName  string
	}{
		{
			name:       "not found",
			categoryID: "cat-999",
			req:        &models.UpdateCategoryRequest{Name: &updatedName},
			setupMocks: func(cr *mocks.MockCategoryRepository) {
				cr.On("GetByID", mock.Anything, "cat-999").Return(nil, errors.New("record not found"))
			},
			expectError:   true,
			expectedError: "not found",
		},
		{
			name:       "success",
			categoryID: "cat-1",
			req: &models.UpdateCategoryRequest{
				Name:  &updatedName,
				Color: &updatedColor,
			},
			setupMocks: func(cr *mocks.MockCategoryRepository) {
				cr.On("GetByID", mock.Anything, "cat-1").Return(testSellCategory("cat-1", "Electronics"), nil)
				cr.On("Update", mock.Anything, mock.AnythingOfType("*models.SellCategory")).Return(nil)
			},
			expectError:  false,
			expectedName: updatedName,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			categoryRepo := new(mocks.MockCategoryRepository)
			tt.setupMocks(categoryRepo)

			svc := newTestCategoryService(categoryRepo)
			resp, err := svc.UpdateCategory(context.Background(), tt.categoryID, tt.req)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, resp)
				assert.Contains(t, strings.ToLower(err.Error()), strings.ToLower(tt.expectedError))
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.Equal(t, tt.expectedName, resp.Name)
			}

			categoryRepo.AssertExpectations(t)
		})
	}
}

// ---------------------------------------------------------------------------
// TestCategoryService_DeleteCategory
// ---------------------------------------------------------------------------

func TestCategoryService_DeleteCategory(t *testing.T) {
	tests := []struct {
		name          string
		categoryID    string
		setupMocks    func(*mocks.MockCategoryRepository)
		expectError   bool
		expectedError string
	}{
		{
			name:       "success",
			categoryID: "cat-1",
			setupMocks: func(cr *mocks.MockCategoryRepository) {
				cr.On("GetByID", mock.Anything, "cat-1").Return(testSellCategory("cat-1", "Electronics"), nil)
				cr.On("Delete", mock.Anything, "cat-1").Return(nil)
			},
			expectError: false,
		},
		{
			name:       "failure — not found",
			categoryID: "cat-999",
			setupMocks: func(cr *mocks.MockCategoryRepository) {
				cr.On("GetByID", mock.Anything, "cat-999").Return(nil, errors.New("record not found"))
			},
			expectError:   true,
			expectedError: "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			categoryRepo := new(mocks.MockCategoryRepository)
			tt.setupMocks(categoryRepo)

			svc := newTestCategoryService(categoryRepo)
			err := svc.DeleteCategory(context.Background(), tt.categoryID)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, strings.ToLower(err.Error()), strings.ToLower(tt.expectedError))
			} else {
				assert.NoError(t, err)
			}

			categoryRepo.AssertExpectations(t)
		})
	}
}

// ---------------------------------------------------------------------------
// TestCategoryService_ListCategories
// ---------------------------------------------------------------------------

func TestCategoryService_ListCategories(t *testing.T) {
	activeStatus := models.CategoryStatusActive

	tests := []struct {
		name          string
		filter        *models.CategoryListFilter
		locale        string
		setupMocks    func(*mocks.MockCategoryRepository)
		expectError   bool
		expectedCount int
	}{
		{
			name: "success with filter",
			filter: &models.CategoryListFilter{
				Status: &activeStatus,
				Limit:  10,
				Offset: 0,
			},
			locale: models.LocaleEN,
			setupMocks: func(cr *mocks.MockCategoryRepository) {
				categories := []*models.SellCategory{
					testSellCategory("cat-1", "Electronics"),
					testSellCategory("cat-2", "Clothing"),
				}
				cr.On("List", mock.Anything, mock.AnythingOfType("*models.CategoryListFilter")).Return(categories, nil)
			},
			expectError:   false,
			expectedCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			categoryRepo := new(mocks.MockCategoryRepository)
			tt.setupMocks(categoryRepo)

			svc := newTestCategoryService(categoryRepo)
			resp, err := svc.ListCategories(context.Background(), tt.filter, tt.locale)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, resp, tt.expectedCount)
			}

			categoryRepo.AssertExpectations(t)
		})
	}
}

// ---------------------------------------------------------------------------
// TestCategoryService_GetCategoriesByIDs
// ---------------------------------------------------------------------------

func TestCategoryService_GetCategoriesByIDs(t *testing.T) {
	tests := []struct {
		name          string
		categoryIDs   []string
		locale        string
		setupMocks    func(*mocks.MockCategoryRepository)
		expectError   bool
		expectedCount int
	}{
		{
			name:        "success",
			categoryIDs: []string{"cat-1", "cat-2"},
			locale:      models.LocaleEN,
			setupMocks: func(cr *mocks.MockCategoryRepository) {
				categories := []*models.SellCategory{
					testSellCategory("cat-1", "Electronics"),
					testSellCategory("cat-2", "Clothing"),
				}
				cr.On("GetByIDs", mock.Anything, []string{"cat-1", "cat-2"}).Return(categories, nil)
			},
			expectError:   false,
			expectedCount: 2,
		},
		{
			name:        "empty IDs — returns early without calling repo",
			categoryIDs: []string{},
			locale:      models.LocaleEN,
			setupMocks:  func(cr *mocks.MockCategoryRepository) {
				// No calls expected; service short-circuits on empty slice
			},
			expectError:   false,
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			categoryRepo := new(mocks.MockCategoryRepository)
			tt.setupMocks(categoryRepo)

			svc := newTestCategoryService(categoryRepo)
			resp, err := svc.GetCategoriesByIDs(context.Background(), tt.categoryIDs, tt.locale)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, resp, tt.expectedCount)
			}

			categoryRepo.AssertExpectations(t)
		})
	}
}
