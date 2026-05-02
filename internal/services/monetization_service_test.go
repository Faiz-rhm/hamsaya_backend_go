package services

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/hamsaya/backend/internal/mocks"
	"github.com/hamsaya/backend/internal/models"
)

func newMonetizationSvc(repo *mocks.MockMonetizationRepository) *MonetizationService {
	return NewMonetizationService(repo, nil, zap.NewNop())
}

// --- ValidateTargetURL ------------------------------------------------------

func TestValidateTargetURL(t *testing.T) {
	cases := []struct {
		name string
		in   string
		ok   bool
	}{
		{"https ok", "https://example.com/landing", true},
		{"http ok", "http://example.com", true},
		{"missing scheme", "example.com", false},
		{"ftp rejected", "ftp://example.com", false},
		{"empty host", "https://", false},
		{"empty", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateTargetURL(c.in)
			if c.ok {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

// --- normalizeAdStatus / normalizeBoostStatus -------------------------------

func TestNormalizeAdStatus(t *testing.T) {
	assert.Equal(t, "PENDING", normalizeAdStatus(" pending "))
	assert.Equal(t, "ACTIVE", normalizeAdStatus("active"))
	assert.Equal(t, "", normalizeAdStatus("garbage"))
	assert.Equal(t, "", normalizeAdStatus(""))
}

func TestNormalizeBoostStatus(t *testing.T) {
	assert.Equal(t, "ACTIVE", normalizeBoostStatus("active"))
	assert.Equal(t, "CANCELLED", normalizeBoostStatus("cancelled"))
	assert.Equal(t, "", normalizeBoostStatus("done"))
}

// --- CreateAd ---------------------------------------------------------------

func TestMonetizationService_CreateAd(t *testing.T) {
	t.Run("invalid url rejected before repo", func(t *testing.T) {
		repo := &mocks.MockMonetizationRepository{}
		svc := newMonetizationSvc(repo)
		ad, err := svc.CreateAd(context.Background(), &models.AdCreateRequest{
			AdvertiserID: "u-1",
			Title:        "T",
			TargetURL:    "not-a-url",
		}, "")
		require.Error(t, err)
		assert.Nil(t, ad)
		repo.AssertNotCalled(t, "CreateAd", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("default status PENDING", func(t *testing.T) {
		repo := &mocks.MockMonetizationRepository{}
		want := &models.Ad{ID: "ad-1", Status: "PENDING"}
		repo.On("CreateAd", mock.Anything, "u-1", "Title", "", "https://cdn/x.png",
			"https://example.com", "PENDING", mock.Anything, mock.Anything).
			Return(want, nil)

		svc := newMonetizationSvc(repo)
		got, err := svc.CreateAd(context.Background(), &models.AdCreateRequest{
			AdvertiserID: "u-1",
			Title:        "Title",
			TargetURL:    "https://example.com",
		}, "https://cdn/x.png")
		require.NoError(t, err)
		assert.Equal(t, want, got)
		repo.AssertExpectations(t)
	})

	t.Run("auto_approve sets ACTIVE", func(t *testing.T) {
		repo := &mocks.MockMonetizationRepository{}
		repo.On("CreateAd", mock.Anything, "u-1", "Title", "body-text", "",
			"https://example.com", "ACTIVE", mock.Anything, mock.Anything).
			Return(&models.Ad{ID: "ad-2", Status: "ACTIVE"}, nil)

		svc := newMonetizationSvc(repo)
		body := "body-text"
		got, err := svc.CreateAd(context.Background(), &models.AdCreateRequest{
			AdvertiserID: "u-1",
			Title:        "Title",
			Body:         &body,
			TargetURL:    "https://example.com",
			AutoApprove:  true,
		}, "")
		require.NoError(t, err)
		assert.Equal(t, "ACTIVE", got.Status)
		repo.AssertExpectations(t)
	})

	t.Run("repo error propagates", func(t *testing.T) {
		repo := &mocks.MockMonetizationRepository{}
		repo.On("CreateAd", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(nil, errors.New("db"))

		svc := newMonetizationSvc(repo)
		_, err := svc.CreateAd(context.Background(), &models.AdCreateRequest{
			AdvertiserID: "u-1",
			Title:        "T",
			TargetURL:    "https://example.com",
		}, "")
		require.Error(t, err)
	})
}

// --- GetAd ------------------------------------------------------------------

func TestMonetizationService_GetAd(t *testing.T) {
	t.Run("not found returns ErrAdNotFound", func(t *testing.T) {
		repo := &mocks.MockMonetizationRepository{}
		repo.On("GetAd", mock.Anything, "missing").Return(nil, nil)

		svc := newMonetizationSvc(repo)
		_, err := svc.GetAd(context.Background(), "missing")
		assert.ErrorIs(t, err, ErrAdNotFound)
	})

	t.Run("repo error propagates", func(t *testing.T) {
		repo := &mocks.MockMonetizationRepository{}
		repo.On("GetAd", mock.Anything, "x").Return(nil, errors.New("db"))

		svc := newMonetizationSvc(repo)
		_, err := svc.GetAd(context.Background(), "x")
		require.Error(t, err)
		assert.NotErrorIs(t, err, ErrAdNotFound)
	})

	t.Run("found returns ad", func(t *testing.T) {
		repo := &mocks.MockMonetizationRepository{}
		want := &models.Ad{ID: "ad-1"}
		repo.On("GetAd", mock.Anything, "ad-1").Return(want, nil)

		svc := newMonetizationSvc(repo)
		got, err := svc.GetAd(context.Background(), "ad-1")
		require.NoError(t, err)
		assert.Equal(t, want, got)
	})
}

// --- Approve / Reject -------------------------------------------------------

func TestMonetizationService_Approve(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		repo := &mocks.MockMonetizationRepository{}
		repo.On("GetAd", mock.Anything, "ad-1").Return(nil, nil)

		svc := newMonetizationSvc(repo)
		_, err := svc.Approve(context.Background(), "ad-1", "admin", &models.AdReviewRequest{})
		assert.ErrorIs(t, err, ErrAdNotFound)
	})

	t.Run("invalid transition from ACTIVE", func(t *testing.T) {
		repo := &mocks.MockMonetizationRepository{}
		repo.On("GetAd", mock.Anything, "ad-1").
			Return(&models.Ad{ID: "ad-1", Status: "ACTIVE"}, nil)

		svc := newMonetizationSvc(repo)
		_, err := svc.Approve(context.Background(), "ad-1", "admin", &models.AdReviewRequest{})
		assert.ErrorIs(t, err, ErrInvalidAdStatus)
	})

	t.Run("PENDING → APPROVED", func(t *testing.T) {
		repo := &mocks.MockMonetizationRepository{}
		repo.On("GetAd", mock.Anything, "ad-1").
			Return(&models.Ad{ID: "ad-1", Status: "PENDING"}, nil)
		repo.On("UpdateAdStatus", mock.Anything, "ad-1", "APPROVED", "admin", mock.Anything).
			Return(&models.Ad{ID: "ad-1", Status: "APPROVED"}, nil)

		svc := newMonetizationSvc(repo)
		got, err := svc.Approve(context.Background(), "ad-1", "admin", &models.AdReviewRequest{})
		require.NoError(t, err)
		assert.Equal(t, "APPROVED", got.Status)
		repo.AssertExpectations(t)
	})

	t.Run("REJECTED → APPROVED allowed", func(t *testing.T) {
		repo := &mocks.MockMonetizationRepository{}
		repo.On("GetAd", mock.Anything, "ad-1").
			Return(&models.Ad{ID: "ad-1", Status: "REJECTED"}, nil)
		repo.On("UpdateAdStatus", mock.Anything, "ad-1", "APPROVED", "admin", mock.Anything).
			Return(&models.Ad{ID: "ad-1", Status: "APPROVED"}, nil)

		svc := newMonetizationSvc(repo)
		_, err := svc.Approve(context.Background(), "ad-1", "admin", &models.AdReviewRequest{})
		require.NoError(t, err)
	})
}

func TestMonetizationService_Reject(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		repo := &mocks.MockMonetizationRepository{}
		repo.On("GetAd", mock.Anything, "ad-1").Return(nil, nil)

		svc := newMonetizationSvc(repo)
		_, err := svc.Reject(context.Background(), "ad-1", "admin", &models.AdReviewRequest{})
		assert.ErrorIs(t, err, ErrAdNotFound)
	})

	t.Run("invalid transition from EXPIRED", func(t *testing.T) {
		repo := &mocks.MockMonetizationRepository{}
		repo.On("GetAd", mock.Anything, "ad-1").
			Return(&models.Ad{ID: "ad-1", Status: "EXPIRED"}, nil)

		svc := newMonetizationSvc(repo)
		_, err := svc.Reject(context.Background(), "ad-1", "admin", &models.AdReviewRequest{})
		assert.ErrorIs(t, err, ErrInvalidAdStatus)
	})

	t.Run("ACTIVE → REJECTED allowed", func(t *testing.T) {
		repo := &mocks.MockMonetizationRepository{}
		repo.On("GetAd", mock.Anything, "ad-1").
			Return(&models.Ad{ID: "ad-1", Status: "ACTIVE"}, nil)
		repo.On("UpdateAdStatus", mock.Anything, "ad-1", "REJECTED", "admin", mock.Anything).
			Return(&models.Ad{ID: "ad-1", Status: "REJECTED"}, nil)

		svc := newMonetizationSvc(repo)
		got, err := svc.Reject(context.Background(), "ad-1", "admin", &models.AdReviewRequest{})
		require.NoError(t, err)
		assert.Equal(t, "REJECTED", got.Status)
	})
}

// --- DeleteAd ---------------------------------------------------------------

func TestMonetizationService_DeleteAd(t *testing.T) {
	repo := &mocks.MockMonetizationRepository{}
	repo.On("DeleteAd", mock.Anything, "ad-1").Return(nil)
	svc := newMonetizationSvc(repo)
	require.NoError(t, svc.DeleteAd(context.Background(), "ad-1"))

	repo2 := &mocks.MockMonetizationRepository{}
	repo2.On("DeleteAd", mock.Anything, "ad-2").Return(errors.New("db"))
	svc2 := newMonetizationSvc(repo2)
	require.Error(t, svc2.DeleteAd(context.Background(), "ad-2"))
}

// --- ListAds / ListActiveAds / impressions+clicks ---------------------------

func TestMonetizationService_ListAds_NormalizesStatus(t *testing.T) {
	repo := &mocks.MockMonetizationRepository{}
	repo.On("ListAds", mock.Anything, "ACTIVE", 1, 20).
		Return([]*models.Ad{{ID: "1"}}, 1, nil)

	svc := newMonetizationSvc(repo)
	ads, total, err := svc.ListAds(context.Background(), " active ", 1, 20)
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, ads, 1)
	repo.AssertExpectations(t)
}

func TestMonetizationService_ListAds_GarbageStatusBecomesEmpty(t *testing.T) {
	repo := &mocks.MockMonetizationRepository{}
	repo.On("ListAds", mock.Anything, "", 1, 20).Return([]*models.Ad{}, 0, nil)

	svc := newMonetizationSvc(repo)
	_, _, err := svc.ListAds(context.Background(), "garbage", 1, 20)
	require.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestMonetizationService_RecordImpressionAndClick(t *testing.T) {
	repo := &mocks.MockMonetizationRepository{}
	repo.On("IncrementAdImpression", mock.Anything, "ad-1").Return(nil)
	repo.On("IncrementAdClick", mock.Anything, "ad-1").Return(nil)
	svc := newMonetizationSvc(repo)
	require.NoError(t, svc.RecordImpression(context.Background(), "ad-1"))
	require.NoError(t, svc.RecordClick(context.Background(), "ad-1"))
}

// --- Credits ---------------------------------------------------------------

func TestMonetizationService_GetUserCredits(t *testing.T) {
	t.Run("user missing", func(t *testing.T) {
		repo := &mocks.MockMonetizationRepository{}
		repo.On("GetBalance", mock.Anything, "u-x").Return(nil, nil)

		svc := newMonetizationSvc(repo)
		_, err := svc.GetUserCredits(context.Background(), "u-x")
		require.Error(t, err)
	})

	t.Run("transaction load error", func(t *testing.T) {
		repo := &mocks.MockMonetizationRepository{}
		repo.On("GetBalance", mock.Anything, "u-1").
			Return(&models.CreditBalance{UserID: "u-1", Balance: 100}, nil)
		repo.On("ListUserTransactions", mock.Anything, "u-1", 50).
			Return(nil, errors.New("db"))

		svc := newMonetizationSvc(repo)
		_, err := svc.GetUserCredits(context.Background(), "u-1")
		require.Error(t, err)
	})

	t.Run("success", func(t *testing.T) {
		repo := &mocks.MockMonetizationRepository{}
		repo.On("GetBalance", mock.Anything, "u-1").
			Return(&models.CreditBalance{UserID: "u-1", Balance: 100}, nil)
		repo.On("ListUserTransactions", mock.Anything, "u-1", 50).
			Return([]*models.CreditTransaction{{ID: "t1", UserID: "u-1", Amount: 50}}, nil)

		svc := newMonetizationSvc(repo)
		out, err := svc.GetUserCredits(context.Background(), "u-1")
		require.NoError(t, err)
		assert.Equal(t, 100, out.Balance.Balance)
		require.Len(t, out.Transactions, 1)
		assert.Equal(t, "t1", out.Transactions[0].ID)
	})
}

func TestMonetizationService_AdjustCredits(t *testing.T) {
	t.Run("zero amount rejected", func(t *testing.T) {
		repo := &mocks.MockMonetizationRepository{}
		svc := newMonetizationSvc(repo)
		_, err := svc.AdjustCredits(context.Background(), "u-1", &models.AdjustCreditsRequest{Amount: 0, Reason: "x"}, "admin")
		require.Error(t, err)
		repo.AssertNotCalled(t, "AdjustCredits", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("repo error propagates", func(t *testing.T) {
		repo := &mocks.MockMonetizationRepository{}
		req := &models.AdjustCreditsRequest{Amount: 50, Reason: "topup"}
		repo.On("AdjustCredits", mock.Anything, "u-1", req, "admin").
			Return(nil, errors.New("insufficient"))
		svc := newMonetizationSvc(repo)
		_, err := svc.AdjustCredits(context.Background(), "u-1", req, "admin")
		require.Error(t, err)
	})

	t.Run("success", func(t *testing.T) {
		repo := &mocks.MockMonetizationRepository{}
		req := &models.AdjustCreditsRequest{Amount: -10, Reason: "refund"}
		repo.On("AdjustCredits", mock.Anything, "u-1", req, "admin").
			Return(&models.CreditBalance{UserID: "u-1", Balance: 90}, nil)
		svc := newMonetizationSvc(repo)
		out, err := svc.AdjustCredits(context.Background(), "u-1", req, "admin")
		require.NoError(t, err)
		assert.Equal(t, 90, out.Balance)
	})
}

// --- Boosts ----------------------------------------------------------------

func TestMonetizationService_CancelBoost(t *testing.T) {
	t.Run("not found / inactive", func(t *testing.T) {
		repo := &mocks.MockMonetizationRepository{}
		repo.On("CancelBoost", mock.Anything, "b-1", "admin", "spam").
			Return(nil, nil)

		svc := newMonetizationSvc(repo)
		_, err := svc.CancelBoost(context.Background(), "b-1", "admin", "spam")
		assert.ErrorIs(t, err, ErrBoostNotFound)
	})

	t.Run("repo error", func(t *testing.T) {
		repo := &mocks.MockMonetizationRepository{}
		repo.On("CancelBoost", mock.Anything, "b-1", "admin", "spam").
			Return(nil, errors.New("db"))
		svc := newMonetizationSvc(repo)
		_, err := svc.CancelBoost(context.Background(), "b-1", "admin", "spam")
		require.Error(t, err)
		assert.NotErrorIs(t, err, ErrBoostNotFound)
	})

	t.Run("success trims reason", func(t *testing.T) {
		repo := &mocks.MockMonetizationRepository{}
		repo.On("CancelBoost", mock.Anything, "b-1", "admin", "spam").
			Return(&models.Boost{ID: "b-1", Status: "CANCELLED"}, nil)
		svc := newMonetizationSvc(repo)
		out, err := svc.CancelBoost(context.Background(), "b-1", "admin", "  spam  ")
		require.NoError(t, err)
		assert.Equal(t, "CANCELLED", out.Status)
	})
}

func TestMonetizationService_ListBoosts_NormalizesStatus(t *testing.T) {
	repo := &mocks.MockMonetizationRepository{}
	repo.On("ListBoosts", mock.Anything, "CANCELLED", 1, 20).
		Return([]*models.Boost{}, 0, nil)

	svc := newMonetizationSvc(repo)
	_, _, err := svc.ListBoosts(context.Background(), "cancelled", 1, 20)
	require.NoError(t, err)
	repo.AssertExpectations(t)
}
