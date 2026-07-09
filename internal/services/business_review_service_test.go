package services

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/hamsaya/backend/internal/mocks"
	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/internal/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func newTestReviewService(
	reviewRepo *mocks.MockBusinessReviewRepository,
	businessRepo *mocks.MockBusinessRepository,
) *BusinessReviewService {
	// notificationService is nil — Submit fires notifyOwner in a goroutine that
	// recovers from any panic, so tests never observe the side-effect.
	return NewBusinessReviewService(reviewRepo, businessRepo, &mocks.MockUserRepository{}, nil, zap.NewNop())
}

func ptrInt(v int) *int       { return &v }
func ptrStr(v string) *string { return &v }

// --- Submit -----------------------------------------------------------------

func TestBusinessReviewService_Submit(t *testing.T) {
	t.Run("business not found", func(t *testing.T) {
		reviewRepo := &mocks.MockBusinessReviewRepository{}
		businessRepo := &mocks.MockBusinessRepository{}
		businessRepo.On("GetByID", mock.Anything, "biz-1").Return(nil, errors.New("nope"))

		svc := newTestReviewService(reviewRepo, businessRepo)
		got, err := svc.Submit(context.Background(), "biz-1", "user-1", &models.CreateBusinessReviewRequest{Rating: 5})

		require.Error(t, err)
		assert.Nil(t, got)
		assert.Contains(t, strings.ToLower(err.Error()), "not found")
		businessRepo.AssertExpectations(t)
	})

	t.Run("self review rejected", func(t *testing.T) {
		reviewRepo := &mocks.MockBusinessReviewRepository{}
		businessRepo := &mocks.MockBusinessRepository{}
		businessRepo.On("GetByID", mock.Anything, "biz-1").
			Return(&models.BusinessProfile{ID: "biz-1", UserID: "user-1"}, nil)

		svc := newTestReviewService(reviewRepo, businessRepo)
		got, err := svc.Submit(context.Background(), "biz-1", "user-1", &models.CreateBusinessReviewRequest{Rating: 4})

		require.Error(t, err)
		assert.Nil(t, got)
		assert.Contains(t, strings.ToLower(err.Error()), "cannot review your own")
		reviewRepo.AssertNotCalled(t, "Upsert", mock.Anything, mock.Anything)
	})

	t.Run("upsert error", func(t *testing.T) {
		reviewRepo := &mocks.MockBusinessReviewRepository{}
		businessRepo := &mocks.MockBusinessRepository{}
		businessRepo.On("GetByID", mock.Anything, "biz-1").
			Return(&models.BusinessProfile{ID: "biz-1", UserID: "owner-9"}, nil)
		reviewRepo.On("Upsert", mock.Anything, mock.AnythingOfType("*models.BusinessReview")).
			Return(errors.New("db down"))

		svc := newTestReviewService(reviewRepo, businessRepo)
		got, err := svc.Submit(context.Background(), "biz-1", "user-1", &models.CreateBusinessReviewRequest{Rating: 4})

		require.Error(t, err)
		assert.Nil(t, got)
		assert.Contains(t, strings.ToLower(err.Error()), "failed to submit review")
		reviewRepo.AssertExpectations(t)
	})

	t.Run("success", func(t *testing.T) {
		reviewRepo := &mocks.MockBusinessReviewRepository{}
		businessRepo := &mocks.MockBusinessRepository{}
		businessRepo.On("GetByID", mock.Anything, "biz-1").
			Return(&models.BusinessProfile{ID: "biz-1", UserID: "owner-9"}, nil)
		reviewRepo.On("Upsert", mock.Anything, mock.MatchedBy(func(r *models.BusinessReview) bool {
			return r.BusinessProfileID == "biz-1" && r.UserID == "user-1" &&
				r.Rating == 5 && r.ID != ""
		})).Return(nil)

		svc := newTestReviewService(reviewRepo, businessRepo)
		got, err := svc.Submit(context.Background(), "biz-1", "user-1",
			&models.CreateBusinessReviewRequest{Rating: 5, Comment: ptrStr("Great!")})

		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, 5, got.Rating)
		assert.Equal(t, "biz-1", got.BusinessProfileID)
		assert.Equal(t, "user-1", got.UserID)
		assert.NotEmpty(t, got.ID)
		reviewRepo.AssertExpectations(t)
	})
}

// --- Update -----------------------------------------------------------------

func TestBusinessReviewService_Update(t *testing.T) {
	t.Run("not found maps to NotFound error", func(t *testing.T) {
		reviewRepo := &mocks.MockBusinessReviewRepository{}
		reviewRepo.On("Update", mock.Anything, "rev-1", "user-1", mock.Anything, mock.Anything).
			Return(nil, repositories.ErrReviewNotFound)

		svc := newTestReviewService(reviewRepo, &mocks.MockBusinessRepository{})
		got, err := svc.Update(context.Background(), "rev-1", "user-1",
			&models.UpdateBusinessReviewRequest{Rating: ptrInt(3)})

		require.Error(t, err)
		assert.Nil(t, got)
		assert.Contains(t, strings.ToLower(err.Error()), "review not found")
	})

	t.Run("repo error", func(t *testing.T) {
		reviewRepo := &mocks.MockBusinessReviewRepository{}
		reviewRepo.On("Update", mock.Anything, "rev-1", "user-1", mock.Anything, mock.Anything).
			Return(nil, errors.New("boom"))

		svc := newTestReviewService(reviewRepo, &mocks.MockBusinessRepository{})
		got, err := svc.Update(context.Background(), "rev-1", "user-1",
			&models.UpdateBusinessReviewRequest{Rating: ptrInt(3)})

		require.Error(t, err)
		assert.Nil(t, got)
		assert.Contains(t, strings.ToLower(err.Error()), "failed to update review")
	})

	t.Run("success", func(t *testing.T) {
		updated := &models.BusinessReview{ID: "rev-1", BusinessProfileID: "biz-1", UserID: "user-1", Rating: 3}
		reviewRepo := &mocks.MockBusinessReviewRepository{}
		reviewRepo.On("Update", mock.Anything, "rev-1", "user-1", mock.Anything, mock.Anything).
			Return(updated, nil)

		// Update now re-notifies the owner (best-effort goroutine) — stub the
		// business lookup it does before spawning the notifier.
		bizRepo := &mocks.MockBusinessRepository{}
		bizRepo.On("GetByID", mock.Anything, "biz-1").
			Return(&models.BusinessProfile{ID: "biz-1", UserID: "owner-1", Name: "Biz"}, nil).Maybe()

		svc := newTestReviewService(reviewRepo, bizRepo)
		got, err := svc.Update(context.Background(), "rev-1", "user-1",
			&models.UpdateBusinessReviewRequest{Rating: ptrInt(3)})

		require.NoError(t, err)
		assert.Equal(t, updated, got)
	})
}

// --- Delete -----------------------------------------------------------------

func TestBusinessReviewService_Delete(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		reviewRepo := &mocks.MockBusinessReviewRepository{}
		reviewRepo.On("Delete", mock.Anything, "rev-1", "user-1", false).
			Return(repositories.ErrReviewNotFound)

		svc := newTestReviewService(reviewRepo, &mocks.MockBusinessRepository{})
		err := svc.Delete(context.Background(), "rev-1", "user-1", false)

		require.Error(t, err)
		assert.Contains(t, strings.ToLower(err.Error()), "review not found")
	})

	t.Run("admin delete passes flag", func(t *testing.T) {
		reviewRepo := &mocks.MockBusinessReviewRepository{}
		reviewRepo.On("Delete", mock.Anything, "rev-1", "admin-9", true).Return(nil)

		svc := newTestReviewService(reviewRepo, &mocks.MockBusinessRepository{})
		err := svc.Delete(context.Background(), "rev-1", "admin-9", true)

		require.NoError(t, err)
		reviewRepo.AssertExpectations(t)
	})

	t.Run("repo error wraps as internal", func(t *testing.T) {
		reviewRepo := &mocks.MockBusinessReviewRepository{}
		reviewRepo.On("Delete", mock.Anything, "rev-1", "user-1", false).
			Return(errors.New("db down"))

		svc := newTestReviewService(reviewRepo, &mocks.MockBusinessRepository{})
		err := svc.Delete(context.Background(), "rev-1", "user-1", false)

		require.Error(t, err)
		assert.Contains(t, strings.ToLower(err.Error()), "failed to delete review")
	})
}

// --- SetHidden --------------------------------------------------------------

func TestBusinessReviewService_SetHidden(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		reviewRepo := &mocks.MockBusinessReviewRepository{}
		reviewRepo.On("SetHidden", mock.Anything, "rev-1", true).
			Return(repositories.ErrReviewNotFound)

		svc := newTestReviewService(reviewRepo, &mocks.MockBusinessRepository{})
		err := svc.SetHidden(context.Background(), "rev-1", true)

		require.Error(t, err)
		assert.Contains(t, strings.ToLower(err.Error()), "review not found")
	})

	t.Run("success", func(t *testing.T) {
		reviewRepo := &mocks.MockBusinessReviewRepository{}
		reviewRepo.On("SetHidden", mock.Anything, "rev-1", true).Return(nil)

		svc := newTestReviewService(reviewRepo, &mocks.MockBusinessRepository{})
		err := svc.SetHidden(context.Background(), "rev-1", true)

		require.NoError(t, err)
	})
}

// --- GetMyReview ------------------------------------------------------------

func TestBusinessReviewService_GetMyReview(t *testing.T) {
	t.Run("repo error", func(t *testing.T) {
		reviewRepo := &mocks.MockBusinessReviewRepository{}
		reviewRepo.On("GetByBusinessAndUser", mock.Anything, "biz-1", "user-1").
			Return(nil, errors.New("db"))

		svc := newTestReviewService(reviewRepo, &mocks.MockBusinessRepository{})
		got, err := svc.GetMyReview(context.Background(), "biz-1", "user-1")

		require.Error(t, err)
		assert.Nil(t, got)
	})

	t.Run("nil when no review", func(t *testing.T) {
		reviewRepo := &mocks.MockBusinessReviewRepository{}
		reviewRepo.On("GetByBusinessAndUser", mock.Anything, "biz-1", "user-1").
			Return(nil, nil)

		svc := newTestReviewService(reviewRepo, &mocks.MockBusinessRepository{})
		got, err := svc.GetMyReview(context.Background(), "biz-1", "user-1")

		require.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("success", func(t *testing.T) {
		want := &models.BusinessReview{ID: "rev-1", BusinessProfileID: "biz-1", UserID: "user-1", Rating: 4}
		reviewRepo := &mocks.MockBusinessReviewRepository{}
		reviewRepo.On("GetByBusinessAndUser", mock.Anything, "biz-1", "user-1").
			Return(want, nil)

		svc := newTestReviewService(reviewRepo, &mocks.MockBusinessRepository{})
		got, err := svc.GetMyReview(context.Background(), "biz-1", "user-1")

		require.NoError(t, err)
		assert.Equal(t, want, got)
	})
}

// --- List -------------------------------------------------------------------

func TestBusinessReviewService_List(t *testing.T) {
	t.Run("clamps invalid limit/offset", func(t *testing.T) {
		reviewRepo := &mocks.MockBusinessReviewRepository{}
		// expect normalized: limit=20, offset=0
		reviewRepo.On("ListByBusiness", mock.Anything, "biz-1", false, 20, 0).
			Return([]*models.BusinessReviewWithAuthor{}, 0, nil)

		svc := newTestReviewService(reviewRepo, &mocks.MockBusinessRepository{})
		_, _, err := svc.List(context.Background(), "biz-1", false, -1, -5)

		require.NoError(t, err)
		reviewRepo.AssertExpectations(t)
	})

	t.Run("clamps limit > 100", func(t *testing.T) {
		reviewRepo := &mocks.MockBusinessReviewRepository{}
		reviewRepo.On("ListByBusiness", mock.Anything, "biz-1", false, 20, 0).
			Return([]*models.BusinessReviewWithAuthor{}, 0, nil)

		svc := newTestReviewService(reviewRepo, &mocks.MockBusinessRepository{})
		_, _, err := svc.List(context.Background(), "biz-1", false, 500, 0)

		require.NoError(t, err)
		reviewRepo.AssertExpectations(t)
	})

	t.Run("includeHidden propagated", func(t *testing.T) {
		reviewRepo := &mocks.MockBusinessReviewRepository{}
		reviewRepo.On("ListByBusiness", mock.Anything, "biz-1", true, 10, 5).
			Return([]*models.BusinessReviewWithAuthor{
				{BusinessReview: models.BusinessReview{ID: "r1", Rating: 5}},
			}, 1, nil)

		svc := newTestReviewService(reviewRepo, &mocks.MockBusinessRepository{})
		items, total, err := svc.List(context.Background(), "biz-1", true, 10, 5)

		require.NoError(t, err)
		assert.Equal(t, 1, total)
		assert.Len(t, items, 1)
	})

	t.Run("repo error", func(t *testing.T) {
		reviewRepo := &mocks.MockBusinessReviewRepository{}
		reviewRepo.On("ListByBusiness", mock.Anything, "biz-1", false, 20, 0).
			Return(nil, 0, errors.New("db"))

		svc := newTestReviewService(reviewRepo, &mocks.MockBusinessRepository{})
		_, _, err := svc.List(context.Background(), "biz-1", false, 0, 0)

		require.Error(t, err)
	})
}

// --- Stats ------------------------------------------------------------------

func TestBusinessReviewService_Stats(t *testing.T) {
	t.Run("repo error", func(t *testing.T) {
		reviewRepo := &mocks.MockBusinessReviewRepository{}
		reviewRepo.On("GetStats", mock.Anything, "biz-1").Return(nil, errors.New("db"))

		svc := newTestReviewService(reviewRepo, &mocks.MockBusinessRepository{})
		got, err := svc.Stats(context.Background(), "biz-1")

		require.Error(t, err)
		assert.Nil(t, got)
	})

	t.Run("success returns stats", func(t *testing.T) {
		want := &models.BusinessReviewStats{
			BusinessProfileID: "biz-1",
			AvgRating:         4.5,
			ReviewCount:       10,
			Distribution:      [5]int{0, 0, 1, 3, 6},
		}
		reviewRepo := &mocks.MockBusinessReviewRepository{}
		reviewRepo.On("GetStats", mock.Anything, "biz-1").Return(want, nil)

		svc := newTestReviewService(reviewRepo, &mocks.MockBusinessRepository{})
		got, err := svc.Stats(context.Background(), "biz-1")

		require.NoError(t, err)
		assert.Equal(t, want, got)
	})
}
