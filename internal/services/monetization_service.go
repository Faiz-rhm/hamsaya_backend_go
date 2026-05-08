package services

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"go.uber.org/zap"

	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/internal/repositories"
)

// MonetizationService is the admin-facing service for ads, credits, and
// boosts. The user-facing surface (advertiser submission, boost purchase) is
// not implemented here — it will live in a separate user-facing service so
// the admin path stays focused on review/oversight semantics.
type MonetizationService struct {
	repo    repositories.MonetizationRepository
	storage *StorageService
	logger  *zap.Logger
}

func NewMonetizationService(
	repo repositories.MonetizationRepository,
	storage *StorageService,
	logger *zap.Logger,
) *MonetizationService {
	return &MonetizationService{repo: repo, storage: storage, logger: logger}
}

// ErrAdNotFound is returned when an ad lookup misses.
var (
	ErrAdNotFound      = errors.New("ad not found")
	ErrInvalidAdStatus = errors.New("invalid ad status transition")
	ErrBoostNotFound   = errors.New("boost not found or already inactive")
)

// ─── Ads ─────────────────────────────────────────────────────────────────────

func (s *MonetizationService) ListAds(ctx context.Context, status string, page, limit int) ([]*models.Ad, int, error) {
	return s.repo.ListAds(ctx, normalizeAdStatus(status), page, limit)
}

// ListActiveAds returns the public-facing slice of ads ready to be served
// inline in the mobile feed. Province + language are user-context hints
// that the repository uses for targeting (empty strings disable targeting
// per dimension).
func (s *MonetizationService) ListActiveAds(ctx context.Context, limit int, province, language string) ([]*models.Ad, error) {
	return s.repo.ListActiveAds(ctx, limit, province, language)
}

func (s *MonetizationService) RecordImpression(ctx context.Context, id string) error {
	return s.repo.IncrementAdImpression(ctx, id)
}

func (s *MonetizationService) RecordClick(ctx context.Context, id string) error {
	return s.repo.IncrementAdClick(ctx, id)
}

// CreateAd inserts a new placement. `imageURL` may be empty when the admin
// did not attach an image. Status defaults to PENDING; AutoApprove flips it
// to ACTIVE for super_admin convenience.
func (s *MonetizationService) CreateAd(
	ctx context.Context,
	req *models.AdCreateRequest,
	imageURL string,
) (*models.Ad, error) {
	if err := ValidateTargetURL(req.TargetURL); err != nil {
		return nil, err
	}
	status := "PENDING"
	if req.AutoApprove {
		status = "ACTIVE"
	}
	body := ""
	if req.Body != nil {
		body = *req.Body
	}
	phone := ""
	if req.PhoneNumber != nil {
		phone = *req.PhoneNumber
	}
	whatsapp := ""
	if req.WhatsAppNumber != nil {
		whatsapp = *req.WhatsAppNumber
	}
	weight := 1
	if req.Weight != nil {
		weight = *req.Weight
	}
	// Manual range guards (validator tags removed — go-playground/validator's
	// `max` tag on *int formats as "must be at most N characters", which is
	// confusing for numeric inputs. We clamp instead of rejecting so a zero
	// or out-of-range weight from the admin UI doesn't 400 the request.
	if weight < 1 {
		weight = 1
	}
	if weight > 100 {
		weight = 100
	}
	if req.DailyImpressionCap != nil && *req.DailyImpressionCap < 1 {
		req.DailyImpressionCap = nil // 0/negative = no cap.
	}
	provinces := req.TargetProvinces
	if provinces == nil {
		provinces = []string{}
	}
	languages := req.TargetLanguages
	if languages == nil {
		languages = []string{}
	}
	return s.repo.CreateAd(ctx, req.AdvertiserID, req.Title, body, imageURL,
		req.TargetURL, phone, whatsapp, status, req.StartAt, req.EndAt,
		weight, req.DailyImpressionCap, provinces, languages)
}

func (s *MonetizationService) GetAd(ctx context.Context, id string) (*models.Ad, error) {
	ad, err := s.repo.GetAd(ctx, id)
	if err != nil {
		return nil, err
	}
	if ad == nil {
		return nil, ErrAdNotFound
	}
	return ad, nil
}

// Approve flips a PENDING ad to APPROVED (or ACTIVE if its start window is in
// the past). Optional start/end fields default to now and now+7d when missing.
func (s *MonetizationService) Approve(ctx context.Context, id, adminID string, req *models.AdReviewRequest) (*models.Ad, error) {
	current, err := s.repo.GetAd(ctx, id)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, ErrAdNotFound
	}
	if current.Status != "PENDING" && current.Status != "REJECTED" {
		return nil, ErrInvalidAdStatus
	}
	target := "APPROVED"
	updated, err := s.repo.UpdateAdStatus(ctx, id, target, adminID, req)
	if err != nil {
		return nil, err
	}
	return updated, nil
}

func (s *MonetizationService) Reject(ctx context.Context, id, adminID string, req *models.AdReviewRequest) (*models.Ad, error) {
	current, err := s.repo.GetAd(ctx, id)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, ErrAdNotFound
	}
	if current.Status != "PENDING" && current.Status != "APPROVED" && current.Status != "ACTIVE" {
		return nil, ErrInvalidAdStatus
	}
	updated, err := s.repo.UpdateAdStatus(ctx, id, "REJECTED", adminID, req)
	if err != nil {
		return nil, err
	}
	return updated, nil
}

func (s *MonetizationService) DeleteAd(ctx context.Context, id string) error {
	if err := s.repo.DeleteAd(ctx, id); err != nil {
		s.logger.Warn("ad delete", zap.String("id", id), zap.Error(err))
		return err
	}
	return nil
}

// ─── Credits ─────────────────────────────────────────────────────────────────

func (s *MonetizationService) ListBalances(ctx context.Context, search string, page, limit int) ([]*models.CreditBalance, int, error) {
	return s.repo.ListBalances(ctx, strings.TrimSpace(search), page, limit)
}

func (s *MonetizationService) GetUserCredits(ctx context.Context, userID string) (*models.CreditsUserDetail, error) {
	balance, err := s.repo.GetBalance(ctx, userID)
	if err != nil {
		return nil, err
	}
	if balance == nil {
		// User does not exist at all.
		return nil, fmt.Errorf("user %s not found", userID)
	}
	tx, err := s.repo.ListUserTransactions(ctx, userID, 50)
	if err != nil {
		return nil, err
	}
	out := &models.CreditsUserDetail{Balance: *balance}
	out.Transactions = make([]models.CreditTransaction, 0, len(tx))
	for _, t := range tx {
		out.Transactions = append(out.Transactions, *t)
	}
	return out, nil
}

func (s *MonetizationService) AdjustCredits(ctx context.Context, userID string, req *models.AdjustCreditsRequest, adminID string) (*models.CreditBalance, error) {
	if req.Amount == 0 {
		return nil, errors.New("amount must be non-zero")
	}
	balance, err := s.repo.AdjustCredits(ctx, userID, req, adminID)
	if err != nil {
		return nil, err
	}
	return balance, nil
}

// ─── Boosts ──────────────────────────────────────────────────────────────────

func (s *MonetizationService) ListBoosts(ctx context.Context, status string, page, limit int) ([]*models.Boost, int, error) {
	return s.repo.ListBoosts(ctx, normalizeBoostStatus(status), page, limit)
}

func (s *MonetizationService) CancelBoost(ctx context.Context, id, adminID, reason string) (*models.Boost, error) {
	boost, err := s.repo.CancelBoost(ctx, id, adminID, strings.TrimSpace(reason))
	if err != nil {
		return nil, err
	}
	if boost == nil {
		return nil, ErrBoostNotFound
	}
	return boost, nil
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func normalizeAdStatus(s string) string {
	s = strings.ToUpper(strings.TrimSpace(s))
	switch s {
	case "PENDING", "APPROVED", "REJECTED", "ACTIVE", "EXPIRED":
		return s
	}
	return ""
}

func normalizeBoostStatus(s string) string {
	s = strings.ToUpper(strings.TrimSpace(s))
	switch s {
	case "ACTIVE", "EXPIRED", "CANCELLED":
		return s
	}
	return ""
}

// ValidateTargetURL is exposed for the (future) advertiser-submission flow but
// kept here so the admin "edit URL on review" path can reuse it.
func ValidateTargetURL(raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return errors.New("url must be http or https")
	}
	if u.Host == "" {
		return errors.New("url must include a host")
	}
	return nil
}
