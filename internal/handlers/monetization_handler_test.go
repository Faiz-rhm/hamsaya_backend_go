package handlers

import (
	"bytes"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"

	"github.com/hamsaya/backend/internal/mocks"
	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/internal/services"
	"github.com/hamsaya/backend/internal/testutil"
)

const (
	monAdminID = "admin-mon-001"
	monAdID    = "ad-mon-001"
)

func newMonetizationRouter(t *testing.T, repo *mocks.MockMonetizationRepository) *gin.Engine {
	t.Helper()
	// storage is nil — these tests never exercise the image-upload path, so
	// the handler never dereferences it.
	svc := services.NewMonetizationService(repo, nil, zap.NewNop())
	// nil redis: handler short-circuits dedupe to "always record" when redis
	// is unavailable, which is the desired test behaviour (we want every
	// impression/click in the test to count).
	h := NewMonetizationHandler(svc, nil, testutil.CreateTestValidator(), zap.NewNop(), nil)

	auth := authContextMiddleware(monAdminID, "sess-mon-001")
	r := gin.New()

	pub := r.Group("/api/v1")
	pub.GET("/ads/active", h.ListActiveAdsPublic)
	pub.POST("/ads/:ad_id/impression", h.RecordAdImpression)
	pub.POST("/ads/:ad_id/click", h.RecordAdClick)

	admin := r.Group("/api/v1/admin")
	admin.Use(auth)
	admin.POST("/ads", h.CreateAd)
	admin.GET("/ads", h.ListAds)
	admin.GET("/ads/:ad_id", h.GetAd)
	admin.POST("/ads/:ad_id/approve", h.ApproveAd)
	admin.POST("/ads/:ad_id/reject", h.RejectAd)
	admin.DELETE("/ads/:ad_id", h.DeleteAd)
	admin.GET("/credits", h.ListBalances)
	admin.GET("/credits/:user_id", h.GetUserCredits)
	admin.POST("/credits/:user_id/adjust", h.AdjustUserCredits)
	admin.GET("/boosts", h.ListBoosts)
	admin.POST("/boosts/:boost_id/cancel", h.CancelBoost)
	return r
}

func multipartForm(t *testing.T, fields map[string]string) (*bytes.Buffer, string) {
	t.Helper()
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	for k, v := range fields {
		_ = mw.WriteField(k, v)
	}
	_ = mw.Close()
	return body, mw.FormDataContentType()
}

// --- Public ads -------------------------------------------------------------

func TestMonetizationHandler_ListActiveAdsPublic(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo := &mocks.MockMonetizationRepository{}
		repo.On("ListActiveAds", mock.Anything, 10).
			Return([]*models.Ad{{ID: "a1"}, {ID: "a2"}}, nil)
		r := newMonetizationRouter(t, repo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/ads/active", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), `"a1"`)
	})

	t.Run("repo error returns 500", func(t *testing.T) {
		repo := &mocks.MockMonetizationRepository{}
		repo.On("ListActiveAds", mock.Anything, 10).Return(nil, errors.New("db"))
		r := newMonetizationRouter(t, repo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/ads/active", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("empty list serializes as items: []", func(t *testing.T) {
		repo := &mocks.MockMonetizationRepository{}
		repo.On("ListActiveAds", mock.Anything, 10).Return(nil, nil)
		r := newMonetizationRouter(t, repo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/ads/active", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), `"items":[]`)
	})
}

func TestMonetizationHandler_RecordImpressionAndClick(t *testing.T) {
	repo := &mocks.MockMonetizationRepository{}
	repo.On("IncrementAdImpression", mock.Anything, monAdID).Return(nil)
	repo.On("IncrementAdClick", mock.Anything, monAdID).Return(errors.New("ignored"))
	r := newMonetizationRouter(t, repo)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/ads/"+monAdID+"/impression", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest(http.MethodPost, "/api/v1/ads/"+monAdID+"/click", nil)
	r.ServeHTTP(w2, req2)
	// Click handler swallows errors and still returns 200 — fire-and-forget.
	assert.Equal(t, http.StatusOK, w2.Code)
}

// --- Admin CreateAd ---------------------------------------------------------

func TestMonetizationHandler_CreateAd(t *testing.T) {
	t.Run("invalid url rejected", func(t *testing.T) {
		repo := &mocks.MockMonetizationRepository{}
		r := newMonetizationRouter(t, repo)

		body, ct := multipartForm(t, map[string]string{
			"title":      "T",
			"target_url": "not-a-url",
		})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/admin/ads", body)
		req.Header.Set("Content-Type", ct)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("title too short fails validation", func(t *testing.T) {
		repo := &mocks.MockMonetizationRepository{}
		r := newMonetizationRouter(t, repo)

		body, ct := multipartForm(t, map[string]string{
			"title":      "X",
			"target_url": "https://example.com",
		})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/admin/ads", body)
		req.Header.Set("Content-Type", ct)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("success defaults advertiser to admin", func(t *testing.T) {
		repo := &mocks.MockMonetizationRepository{}
		repo.On("CreateAd", mock.Anything, monAdminID, "Hello Ad", "", "",
			"https://example.com", "", "", "PENDING", mock.Anything, mock.Anything).
			Return(&models.Ad{ID: "ad-x", Status: "PENDING"}, nil)
		r := newMonetizationRouter(t, repo)

		body, ct := multipartForm(t, map[string]string{
			"title":      "Hello Ad",
			"target_url": "https://example.com",
		})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/admin/ads", body)
		req.Header.Set("Content-Type", ct)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
		repo.AssertExpectations(t)
	})
}

// --- Admin ListAds / GetAd --------------------------------------------------

func TestMonetizationHandler_ListAds(t *testing.T) {
	repo := &mocks.MockMonetizationRepository{}
	repo.On("ListAds", mock.Anything, "PENDING", 1, 20).
		Return([]*models.Ad{{ID: "a1"}}, 1, nil)
	r := newMonetizationRouter(t, repo)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/admin/ads?status=pending&page=1&limit=20", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"total_count":1`)
}

func TestMonetizationHandler_GetAd(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		repo := &mocks.MockMonetizationRepository{}
		repo.On("GetAd", mock.Anything, "missing").Return(nil, nil)
		r := newMonetizationRouter(t, repo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/admin/ads/missing", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("success", func(t *testing.T) {
		repo := &mocks.MockMonetizationRepository{}
		repo.On("GetAd", mock.Anything, monAdID).Return(&models.Ad{ID: monAdID}, nil)
		r := newMonetizationRouter(t, repo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/admin/ads/"+monAdID, nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("repo error 500", func(t *testing.T) {
		repo := &mocks.MockMonetizationRepository{}
		repo.On("GetAd", mock.Anything, monAdID).Return(nil, errors.New("db"))
		r := newMonetizationRouter(t, repo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/admin/ads/"+monAdID, nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

// --- Admin Approve / Reject -------------------------------------------------

func TestMonetizationHandler_ApproveAd(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		repo := &mocks.MockMonetizationRepository{}
		repo.On("GetAd", mock.Anything, monAdID).Return(nil, nil)
		r := newMonetizationRouter(t, repo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/admin/ads/"+monAdID+"/approve",
			strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("invalid transition", func(t *testing.T) {
		repo := &mocks.MockMonetizationRepository{}
		repo.On("GetAd", mock.Anything, monAdID).
			Return(&models.Ad{ID: monAdID, Status: "ACTIVE"}, nil)
		r := newMonetizationRouter(t, repo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/admin/ads/"+monAdID+"/approve",
			strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("success", func(t *testing.T) {
		repo := &mocks.MockMonetizationRepository{}
		repo.On("GetAd", mock.Anything, monAdID).
			Return(&models.Ad{ID: monAdID, Status: "PENDING"}, nil)
		repo.On("UpdateAdStatus", mock.Anything, monAdID, "APPROVED", monAdminID, mock.Anything).
			Return(&models.Ad{ID: monAdID, Status: "APPROVED"}, nil)
		r := newMonetizationRouter(t, repo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/admin/ads/"+monAdID+"/approve",
			strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), `"APPROVED"`)
	})
}

func TestMonetizationHandler_RejectAd(t *testing.T) {
	repo := &mocks.MockMonetizationRepository{}
	repo.On("GetAd", mock.Anything, monAdID).
		Return(&models.Ad{ID: monAdID, Status: "PENDING"}, nil)
	repo.On("UpdateAdStatus", mock.Anything, monAdID, "REJECTED", monAdminID, mock.Anything).
		Return(&models.Ad{ID: monAdID, Status: "REJECTED"}, nil)
	r := newMonetizationRouter(t, repo)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/admin/ads/"+monAdID+"/reject",
		strings.NewReader(`{"note":"spam"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// --- Admin DeleteAd ---------------------------------------------------------

func TestMonetizationHandler_DeleteAd(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo := &mocks.MockMonetizationRepository{}
		repo.On("DeleteAd", mock.Anything, monAdID).Return(nil)
		r := newMonetizationRouter(t, repo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodDelete, "/api/v1/admin/ads/"+monAdID, nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("repo error", func(t *testing.T) {
		repo := &mocks.MockMonetizationRepository{}
		repo.On("DeleteAd", mock.Anything, monAdID).Return(errors.New("db"))
		r := newMonetizationRouter(t, repo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodDelete, "/api/v1/admin/ads/"+monAdID, nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

// --- Admin Credits ----------------------------------------------------------

func TestMonetizationHandler_ListBalances(t *testing.T) {
	repo := &mocks.MockMonetizationRepository{}
	repo.On("ListBalances", mock.Anything, "alice", 1, 20).
		Return([]*models.CreditBalance{{UserID: "u-1", Balance: 50}}, 1, nil)
	r := newMonetizationRouter(t, repo)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/admin/credits?search=alice", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestMonetizationHandler_GetUserCredits(t *testing.T) {
	t.Run("missing user → 404", func(t *testing.T) {
		repo := &mocks.MockMonetizationRepository{}
		repo.On("GetBalance", mock.Anything, "u-x").Return(nil, nil)
		r := newMonetizationRouter(t, repo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/admin/credits/u-x", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("success", func(t *testing.T) {
		repo := &mocks.MockMonetizationRepository{}
		repo.On("GetBalance", mock.Anything, "u-1").
			Return(&models.CreditBalance{UserID: "u-1", Balance: 50}, nil)
		repo.On("ListUserTransactions", mock.Anything, "u-1", 50).
			Return([]*models.CreditTransaction{}, nil)
		r := newMonetizationRouter(t, repo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/admin/credits/u-1", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestMonetizationHandler_AdjustUserCredits(t *testing.T) {
	t.Run("invalid JSON", func(t *testing.T) {
		repo := &mocks.MockMonetizationRepository{}
		r := newMonetizationRouter(t, repo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/admin/credits/u-1/adjust",
			strings.NewReader(`bad`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("validation: amount=0 fails (required)", func(t *testing.T) {
		repo := &mocks.MockMonetizationRepository{}
		r := newMonetizationRouter(t, repo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/admin/credits/u-1/adjust",
			strings.NewReader(`{"amount":0,"reason":"topup"}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("success", func(t *testing.T) {
		repo := &mocks.MockMonetizationRepository{}
		repo.On("AdjustCredits", mock.Anything, "u-1",
			mock.AnythingOfType("*models.AdjustCreditsRequest"), monAdminID).
			Return(&models.CreditBalance{UserID: "u-1", Balance: 100}, nil)
		r := newMonetizationRouter(t, repo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/admin/credits/u-1/adjust",
			strings.NewReader(`{"amount":50,"reason":"topup"}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

// --- Admin Boosts -----------------------------------------------------------

func TestMonetizationHandler_ListBoosts(t *testing.T) {
	repo := &mocks.MockMonetizationRepository{}
	repo.On("ListBoosts", mock.Anything, "ACTIVE", 1, 20).
		Return([]*models.Boost{{ID: "b1"}}, 1, nil)
	r := newMonetizationRouter(t, repo)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/admin/boosts?status=active", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestMonetizationHandler_CancelBoost(t *testing.T) {
	t.Run("invalid JSON", func(t *testing.T) {
		repo := &mocks.MockMonetizationRepository{}
		r := newMonetizationRouter(t, repo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/admin/boosts/b1/cancel",
			strings.NewReader(`x`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("missing reason fails validation", func(t *testing.T) {
		repo := &mocks.MockMonetizationRepository{}
		r := newMonetizationRouter(t, repo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/admin/boosts/b1/cancel",
			strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("not found", func(t *testing.T) {
		repo := &mocks.MockMonetizationRepository{}
		repo.On("CancelBoost", mock.Anything, "b1", monAdminID, "spam").
			Return(nil, nil)
		r := newMonetizationRouter(t, repo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/admin/boosts/b1/cancel",
			strings.NewReader(`{"reason":"spam"}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("success", func(t *testing.T) {
		repo := &mocks.MockMonetizationRepository{}
		repo.On("CancelBoost", mock.Anything, "b1", monAdminID, "spam").
			Return(&models.Boost{ID: "b1", Status: "CANCELLED"}, nil)
		r := newMonetizationRouter(t, repo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/admin/boosts/b1/cancel",
			strings.NewReader(`{"reason":"spam"}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), `"CANCELLED"`)
	})
}
