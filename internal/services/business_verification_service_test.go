package services

import (
	"context"
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

func newTestVerificationService(
	verificationRepo *mocks.MockBusinessVerificationRepository,
	businessRepo *mocks.MockBusinessRepository,
) *BusinessVerificationService {
	// notification service is nil — notifyOwner no-ops without it, so tests
	// never observe the side-effect.
	return NewBusinessVerificationService(verificationRepo, businessRepo, nil, zap.NewNop())
}

func docPhotos(n int) []models.Photo {
	photos := make([]models.Photo, n)
	for i := range photos {
		photos[i] = models.Photo{URL: "https://cdn/doc.webp"}
	}
	return photos
}

// --- Submit -----------------------------------------------------------------

func TestBusinessVerificationService_Submit(t *testing.T) {
	t.Run("business not found", func(t *testing.T) {
		bizRepo := &mocks.MockBusinessRepository{}
		bizRepo.On("GetByID", mock.Anything, "biz-1").
			Return(nil, repositories.ErrVerificationNotFound)

		svc := newTestVerificationService(&mocks.MockBusinessVerificationRepository{}, bizRepo)
		_, err := svc.Submit(context.Background(), "biz-1", "user-1", nil, nil, docPhotos(1))

		require.Error(t, err)
		assert.Contains(t, strings.ToLower(err.Error()), "not found")
	})

	t.Run("not the owner", func(t *testing.T) {
		bizRepo := &mocks.MockBusinessRepository{}
		bizRepo.On("GetByID", mock.Anything, "biz-1").
			Return(&models.BusinessProfile{ID: "biz-1", UserID: "owner-1"}, nil)

		svc := newTestVerificationService(&mocks.MockBusinessVerificationRepository{}, bizRepo)
		_, err := svc.Submit(context.Background(), "biz-1", "intruder", nil, nil, docPhotos(1))

		require.Error(t, err)
		assert.Contains(t, strings.ToLower(err.Error()), "own")
	})

	t.Run("already verified", func(t *testing.T) {
		bizRepo := &mocks.MockBusinessRepository{}
		bizRepo.On("GetByID", mock.Anything, "biz-1").
			Return(&models.BusinessProfile{ID: "biz-1", UserID: "owner-1", IsVerified: true}, nil)

		svc := newTestVerificationService(&mocks.MockBusinessVerificationRepository{}, bizRepo)
		_, err := svc.Submit(context.Background(), "biz-1", "owner-1", nil, nil, docPhotos(1))

		require.Error(t, err)
		assert.Contains(t, strings.ToLower(err.Error()), "already verified")
	})

	t.Run("no documents", func(t *testing.T) {
		bizRepo := &mocks.MockBusinessRepository{}
		bizRepo.On("GetByID", mock.Anything, "biz-1").
			Return(&models.BusinessProfile{ID: "biz-1", UserID: "owner-1"}, nil)

		svc := newTestVerificationService(&mocks.MockBusinessVerificationRepository{}, bizRepo)
		_, err := svc.Submit(context.Background(), "biz-1", "owner-1", nil, nil, nil)

		require.Error(t, err)
		assert.Contains(t, strings.ToLower(err.Error()), "document")
	})

	t.Run("already pending", func(t *testing.T) {
		bizRepo := &mocks.MockBusinessRepository{}
		bizRepo.On("GetByID", mock.Anything, "biz-1").
			Return(&models.BusinessProfile{ID: "biz-1", UserID: "owner-1"}, nil)
		verRepo := &mocks.MockBusinessVerificationRepository{}
		verRepo.On("Create", mock.Anything, mock.Anything).
			Return(repositories.ErrVerificationPending)

		svc := newTestVerificationService(verRepo, bizRepo)
		_, err := svc.Submit(context.Background(), "biz-1", "owner-1", nil, nil, docPhotos(1))

		require.Error(t, err)
		assert.Contains(t, strings.ToLower(err.Error()), "pending")
	})

	t.Run("success", func(t *testing.T) {
		bizRepo := &mocks.MockBusinessRepository{}
		bizRepo.On("GetByID", mock.Anything, "biz-1").
			Return(&models.BusinessProfile{ID: "biz-1", UserID: "owner-1"}, nil)
		created := &models.BusinessVerificationRequest{
			ID: "req-1", BusinessID: "biz-1", UserID: "owner-1",
			Status: models.VerificationStatusPending,
		}
		verRepo := &mocks.MockBusinessVerificationRepository{}
		verRepo.On("Create", mock.Anything, mock.MatchedBy(func(r *models.BusinessVerificationRequest) bool {
			return r.BusinessID == "biz-1" && r.UserID == "owner-1" && len(r.Documents) == 2
		})).Return(nil)
		verRepo.On("GetLatestByBusiness", mock.Anything, "biz-1").Return(created, nil)

		svc := newTestVerificationService(verRepo, bizRepo)
		got, err := svc.Submit(context.Background(), "biz-1", "owner-1", nil, nil, docPhotos(2))

		require.NoError(t, err)
		assert.Equal(t, "req-1", got.ID)
		verRepo.AssertExpectations(t)
	})
}

// --- Status -----------------------------------------------------------------

func TestBusinessVerificationService_Status(t *testing.T) {
	t.Run("not the owner", func(t *testing.T) {
		bizRepo := &mocks.MockBusinessRepository{}
		bizRepo.On("GetByID", mock.Anything, "biz-1").
			Return(&models.BusinessProfile{ID: "biz-1", UserID: "owner-1"}, nil)

		svc := newTestVerificationService(&mocks.MockBusinessVerificationRepository{}, bizRepo)
		_, err := svc.Status(context.Background(), "biz-1", "intruder")

		require.Error(t, err)
	})

	t.Run("never submitted returns nil", func(t *testing.T) {
		bizRepo := &mocks.MockBusinessRepository{}
		bizRepo.On("GetByID", mock.Anything, "biz-1").
			Return(&models.BusinessProfile{ID: "biz-1", UserID: "owner-1"}, nil)
		verRepo := &mocks.MockBusinessVerificationRepository{}
		verRepo.On("GetLatestByBusiness", mock.Anything, "biz-1").
			Return(nil, repositories.ErrVerificationNotFound)

		svc := newTestVerificationService(verRepo, bizRepo)
		got, err := svc.Status(context.Background(), "biz-1", "owner-1")

		require.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("returns latest", func(t *testing.T) {
		bizRepo := &mocks.MockBusinessRepository{}
		bizRepo.On("GetByID", mock.Anything, "biz-1").
			Return(&models.BusinessProfile{ID: "biz-1", UserID: "owner-1"}, nil)
		latest := &models.BusinessVerificationRequest{ID: "req-1", Status: models.VerificationStatusPending}
		verRepo := &mocks.MockBusinessVerificationRepository{}
		verRepo.On("GetLatestByBusiness", mock.Anything, "biz-1").Return(latest, nil)

		svc := newTestVerificationService(verRepo, bizRepo)
		got, err := svc.Status(context.Background(), "biz-1", "owner-1")

		require.NoError(t, err)
		assert.Equal(t, "req-1", got.ID)
	})
}

// --- Review -----------------------------------------------------------------

func TestBusinessVerificationService_Review(t *testing.T) {
	pendingReq := func() *models.BusinessVerificationRequest {
		return &models.BusinessVerificationRequest{
			ID: "req-1", BusinessID: "biz-1", UserID: "owner-1",
			Status: models.VerificationStatusPending,
		}
	}

	t.Run("not found", func(t *testing.T) {
		verRepo := &mocks.MockBusinessVerificationRepository{}
		verRepo.On("GetByID", mock.Anything, "req-1").
			Return(nil, repositories.ErrVerificationNotFound)

		svc := newTestVerificationService(verRepo, &mocks.MockBusinessRepository{})
		_, err := svc.Review(context.Background(), "req-1", "admin-1", "approve", nil)

		require.Error(t, err)
		assert.Contains(t, strings.ToLower(err.Error()), "not found")
	})

	t.Run("already reviewed", func(t *testing.T) {
		reviewed := pendingReq()
		reviewed.Status = models.VerificationStatusApproved
		verRepo := &mocks.MockBusinessVerificationRepository{}
		verRepo.On("GetByID", mock.Anything, "req-1").Return(reviewed, nil)

		svc := newTestVerificationService(verRepo, &mocks.MockBusinessRepository{})
		_, err := svc.Review(context.Background(), "req-1", "admin-1", "approve", nil)

		require.Error(t, err)
		assert.Contains(t, strings.ToLower(err.Error()), "already")
	})

	t.Run("approve flips the tick", func(t *testing.T) {
		verRepo := &mocks.MockBusinessVerificationRepository{}
		verRepo.On("GetByID", mock.Anything, "req-1").Return(pendingReq(), nil)
		verRepo.On("Review", mock.Anything, "req-1", "admin-1", models.VerificationStatusApproved, (*string)(nil)).
			Return(nil)
		verRepo.On("SetBusinessVerified", mock.Anything, "biz-1", true).Return(nil)

		bizRepo := &mocks.MockBusinessRepository{}
		bizRepo.On("GetByID", mock.Anything, "biz-1").
			Return(&models.BusinessProfile{ID: "biz-1", UserID: "owner-1", Name: "Biz"}, nil).Maybe()

		svc := newTestVerificationService(verRepo, bizRepo)
		_, err := svc.Review(context.Background(), "req-1", "admin-1", "approve", nil)

		require.NoError(t, err)
		verRepo.AssertCalled(t, "SetBusinessVerified", mock.Anything, "biz-1", true)
	})

	t.Run("reject does not flip the tick", func(t *testing.T) {
		reason := "blurry documents"
		verRepo := &mocks.MockBusinessVerificationRepository{}
		verRepo.On("GetByID", mock.Anything, "req-1").Return(pendingReq(), nil)
		verRepo.On("Review", mock.Anything, "req-1", "admin-1", models.VerificationStatusRejected, &reason).
			Return(nil)

		bizRepo := &mocks.MockBusinessRepository{}
		bizRepo.On("GetByID", mock.Anything, "biz-1").
			Return(&models.BusinessProfile{ID: "biz-1", UserID: "owner-1", Name: "Biz"}, nil).Maybe()

		svc := newTestVerificationService(verRepo, bizRepo)
		_, err := svc.Review(context.Background(), "req-1", "admin-1", "reject", &reason)

		require.NoError(t, err)
		verRepo.AssertNotCalled(t, "SetBusinessVerified", mock.Anything, mock.Anything, mock.Anything)
	})
}
