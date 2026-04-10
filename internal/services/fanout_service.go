package services

import (
	"context"

	"github.com/hamsaya/backend/internal/repositories"
	"go.uber.org/zap"
)

// FanoutService manages hybrid push/pull feed fanout.
type FanoutService struct {
	fanoutRepo repositories.FanoutRepository
	logger     *zap.Logger
}

// NewFanoutService creates a new FanoutService.
func NewFanoutService(fanoutRepo repositories.FanoutRepository, logger *zap.Logger) *FanoutService {
	return &FanoutService{fanoutRepo: fanoutRepo, logger: logger}
}

// FanoutPost is called in a background goroutine immediately after a post is
// persisted. It inserts the post into every follower's user_feeds row.
//
// Celebrity authors (> CelebrityThreshold followers) are skipped because their
// posts are queried on read via GetCelebrityPostIDs to avoid write-amplification.
func (s *FanoutService) FanoutPost(ctx context.Context, postID, authorID string) {
	count, err := s.fanoutRepo.CountFollowers(ctx, authorID)
	if err != nil {
		s.logger.Error("FanoutPost: count followers", zap.String("author_id", authorID), zap.Error(err))
		return
	}
	if count > repositories.CelebrityThreshold {
		s.logger.Info("FanoutPost: celebrity author, skipping fanout",
			zap.String("author_id", authorID), zap.Int("followers", count))
		return
	}
	ids, err := s.fanoutRepo.GetFollowerIDs(ctx, authorID)
	if err != nil {
		s.logger.Error("FanoutPost: get follower IDs", zap.String("author_id", authorID), zap.Error(err))
		return
	}
	if err := s.fanoutRepo.InsertFeedEntries(ctx, postID, ids); err != nil {
		s.logger.Error("FanoutPost: insert feed entries", zap.String("post_id", postID), zap.Error(err))
	}
}
