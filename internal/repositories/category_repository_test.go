package repositories_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/internal/repositories"
	"github.com/hamsaya/backend/internal/testutil"
)

func newCategoryRepo(pool *testutil.MockPool) repositories.CategoryRepository {
	return repositories.NewCategoryRepository(testutil.NewTestDB(pool))
}

func testCategory() *models.SellCategory {
	return &models.SellCategory{
		ID:        "cat-1",
		Name:      "Electronics",
		Icon:      models.CategoryIcon{Name: "laptop", Library: "material"},
		Color:     "#FF0000",
		Status:    models.CategoryStatusActive,
		CreatedAt: time.Now(),
	}
}

// makeCategoryScanFn returns a scan function for a single SellCategory row.
// Scan order: id, name, name_dari, name_pashto, icon([]byte), color, status, created_at
func makeCategoryScanFn(cat *models.SellCategory) func(dest ...any) error {
	return func(dest ...any) error {
		iconJSON, _ := json.Marshal(cat.Icon)
		values := []any{cat.ID, cat.Name, cat.NameDari, cat.NamePashto, iconJSON, cat.Color, string(cat.Status), cat.CreatedAt}
		for i, d := range dest {
			if i >= len(values) {
				break
			}
			switch dp := d.(type) {
			case *string:
				if s, ok := values[i].(string); ok {
					*dp = s
				}
			case **string:
				if values[i] == nil {
					*dp = nil
				} else if s, ok := values[i].(string); ok {
					*dp = &s
				}
			case *[]byte:
				if b, ok := values[i].([]byte); ok {
					*dp = b
				}
			case *models.CategoryStatus:
				if s, ok := values[i].(string); ok {
					*dp = models.CategoryStatus(s)
				}
			case *time.Time:
				if t, ok := values[i].(time.Time); ok {
					*dp = t
				}
			}
		}
		return nil
	}
}

func TestCategoryRepository_Create(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		pool := new(testutil.MockPool)
		repo := newCategoryRepo(pool)

		pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
			Return(pgconn.CommandTag{}, nil)

		err := repo.Create(context.Background(), testCategory())

		require.NoError(t, err)
		pool.AssertExpectations(t)
	})

	t.Run("propagates error", func(t *testing.T) {
		pool := new(testutil.MockPool)
		repo := newCategoryRepo(pool)

		pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
			Return(pgconn.CommandTag{}, fmt.Errorf("db error"))

		err := repo.Create(context.Background(), testCategory())

		require.Error(t, err)
	})
}

func TestCategoryRepository_GetByID(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		pool := new(testutil.MockPool)
		repo := newCategoryRepo(pool)

		cat := testCategory()
		pool.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
			Return(testutil.NewMockRow(makeCategoryScanFn(cat)))

		result, err := repo.GetByID(context.Background(), "cat-1")

		require.NoError(t, err)
		assert.Equal(t, "cat-1", result.ID)
		assert.Equal(t, "Electronics", result.Name)
	})

	t.Run("not found", func(t *testing.T) {
		pool := new(testutil.MockPool)
		repo := newCategoryRepo(pool)

		pool.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
			Return(testutil.ErrRow(pgx.ErrNoRows))

		_, err := repo.GetByID(context.Background(), "missing")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestCategoryRepository_GetAll(t *testing.T) {
	t.Run("returns all categories", func(t *testing.T) {
		pool := new(testutil.MockPool)
		repo := newCategoryRepo(pool)

		cat1 := testCategory()
		cat2 := &models.SellCategory{
			ID:        "cat-2",
			Name:      "Clothing",
			Icon:      models.CategoryIcon{Name: "shirt", Library: "material"},
			Color:     "#00FF00",
			Status:    models.CategoryStatusActive,
			CreatedAt: time.Now(),
		}

		rows := testutil.NewFuncRows(makeCategoryScanFn(cat1), makeCategoryScanFn(cat2))
		pool.On("Query", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
			Return(rows, nil)

		results, err := repo.GetAll(context.Background())

		require.NoError(t, err)
		assert.Len(t, results, 2)
		assert.Equal(t, "cat-1", results[0].ID)
		assert.Equal(t, "cat-2", results[1].ID)
	})

	t.Run("returns empty list when no categories", func(t *testing.T) {
		pool := new(testutil.MockPool)
		repo := newCategoryRepo(pool)

		pool.On("Query", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
			Return(testutil.EmptyRows(), nil)

		results, err := repo.GetAll(context.Background())

		require.NoError(t, err)
		assert.Empty(t, results)
	})

	t.Run("propagates query error", func(t *testing.T) {
		pool := new(testutil.MockPool)
		repo := newCategoryRepo(pool)

		pool.On("Query", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
			Return(nil, fmt.Errorf("db error"))

		_, err := repo.GetAll(context.Background())

		require.Error(t, err)
	})
}

func TestCategoryRepository_Delete(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		pool := new(testutil.MockPool)
		repo := newCategoryRepo(pool)

		pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
			Return(pgconn.NewCommandTag("DELETE 1"), nil)

		err := repo.Delete(context.Background(), "cat-1")

		require.NoError(t, err)
	})

	t.Run("not found returns error", func(t *testing.T) {
		pool := new(testutil.MockPool)
		repo := newCategoryRepo(pool)

		pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
			Return(pgconn.NewCommandTag("DELETE 0"), nil)

		err := repo.Delete(context.Background(), "missing")

		require.Error(t, err)
	})
}

func TestCategoryRepository_Update(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		pool := new(testutil.MockPool)
		repo := newCategoryRepo(pool)

		pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
			Return(pgconn.NewCommandTag("UPDATE 1"), nil)

		err := repo.Update(context.Background(), testCategory())

		require.NoError(t, err)
	})

	t.Run("propagates error", func(t *testing.T) {
		pool := new(testutil.MockPool)
		repo := newCategoryRepo(pool)

		pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
			Return(pgconn.CommandTag{}, fmt.Errorf("db error"))

		err := repo.Update(context.Background(), testCategory())

		require.Error(t, err)
	})
}
