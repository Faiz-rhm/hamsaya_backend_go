package services

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/internal/repositories"
	"github.com/hamsaya/backend/internal/utils"
	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"
)

// CommentService handles comment operations
type CommentService struct {
	commentRepo repositories.CommentRepository
	postRepo    repositories.PostRepository
	userRepo    repositories.UserRepository
	logger      *zap.Logger
}

// NewCommentService creates a new comment service
func NewCommentService(
	commentRepo repositories.CommentRepository,
	postRepo repositories.PostRepository,
	userRepo repositories.UserRepository,
	logger *zap.Logger,
) *CommentService {
	return &CommentService{
		commentRepo: commentRepo,
		postRepo:    postRepo,
		userRepo:    userRepo,
		logger:      logger,
	}
}

// CreateComment creates a new comment
func (s *CommentService) CreateComment(ctx context.Context, postID, userID string, req *models.CreateCommentRequest) (*models.CommentResponse, error) {
	// Validate post exists
	_, err := s.postRepo.GetByID(ctx, postID)
	if err != nil {
		return nil, utils.NewNotFoundError("Post not found", err)
	}

	// If replying to a comment, validate parent comment exists
	if req.ParentCommentID != nil {
		parentComment, err := s.commentRepo.GetByID(ctx, *req.ParentCommentID)
		if err != nil {
			return nil, utils.NewNotFoundError("Parent comment not found", err)
		}
		// Ensure parent comment belongs to the same post
		if parentComment.PostID != postID {
			return nil, utils.NewBadRequestError("Parent comment does not belong to this post", nil)
		}
	}

	// Create comment
	commentID := uuid.New().String()
	now := time.Now()

	comment := &models.PostComment{
		ID:              commentID,
		PostID:          postID,
		UserID:          userID,
		ParentCommentID: req.ParentCommentID,
		Text:            req.Text,
		TotalLikes:      0,
		TotalReplies:    0,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	// Handle location
	if req.Latitude != nil && req.Longitude != nil {
		comment.Location = &pgtype.Point{
			P:     pgtype.Vec2{X: *req.Longitude, Y: *req.Latitude},
			Valid: true,
		}
	}

	// Create comment in database
	if err := s.commentRepo.Create(ctx, comment); err != nil {
		s.logger.Error("Failed to create comment", zap.String("post_id", postID), zap.Error(err))
		return nil, utils.NewInternalError("Failed to create comment", err)
	}

	// Create attachments if provided
	if len(req.Attachments) > 0 {
		for _, photoURL := range req.Attachments {
			attachment := &models.CommentAttachment{
				ID:        uuid.New().String(),
				CommentID: commentID,
				Photo: models.Photo{
					URL: photoURL,
					// TODO: Get other photo metadata from storage service
				},
				CreatedAt: now,
				UpdatedAt: now,
			}

			if err := s.commentRepo.CreateAttachment(ctx, attachment); err != nil {
				s.logger.Error("Failed to create comment attachment",
					zap.String("comment_id", commentID),
					zap.Error(err),
				)
				// Continue with other attachments
			}
		}
	}

	s.logger.Info("Comment created",
		zap.String("comment_id", commentID),
		zap.String("post_id", postID),
		zap.String("user_id", userID),
	)

	// Return enriched comment
	return s.GetComment(ctx, commentID, &userID)
}

// GetComment gets a comment by ID with full details
func (s *CommentService) GetComment(ctx context.Context, commentID string, viewerID *string) (*models.CommentResponse, error) {
	// Get comment
	comment, err := s.commentRepo.GetByID(ctx, commentID)
	if err != nil {
		s.logger.Warn("Comment not found", zap.String("comment_id", commentID), zap.Error(err))
		return nil, utils.NewNotFoundError("Comment not found", err)
	}

	// Enrich comment
	return s.enrichComment(ctx, comment, viewerID, false)
}

// GetPostComments gets comments for a post
func (s *CommentService) GetPostComments(ctx context.Context, postID string, limit, offset int, viewerID *string) ([]*models.CommentResponse, error) {
	// Validate post exists
	_, err := s.postRepo.GetByID(ctx, postID)
	if err != nil {
		return nil, utils.NewNotFoundError("Post not found", err)
	}

	// Get top-level comments
	comments, err := s.commentRepo.GetByPostID(ctx, postID, limit, offset)
	if err != nil {
		s.logger.Error("Failed to get post comments", zap.String("post_id", postID), zap.Error(err))
		return nil, utils.NewInternalError("Failed to get comments", err)
	}

	// Enrich comments
	var enrichedComments []*models.CommentResponse
	for _, comment := range comments {
		enrichedComment, err := s.enrichComment(ctx, comment, viewerID, true)
		if err != nil {
			s.logger.Warn("Failed to enrich comment", zap.String("comment_id", comment.ID), zap.Error(err))
			continue
		}
		enrichedComments = append(enrichedComments, enrichedComment)
	}

	return enrichedComments, nil
}

// GetCommentReplies gets replies to a comment
func (s *CommentService) GetCommentReplies(ctx context.Context, commentID string, limit, offset int, viewerID *string) ([]*models.CommentResponse, error) {
	// Validate parent comment exists
	_, err := s.commentRepo.GetByID(ctx, commentID)
	if err != nil {
		return nil, utils.NewNotFoundError("Comment not found", err)
	}

	// Get replies
	replies, err := s.commentRepo.GetReplies(ctx, commentID, limit, offset)
	if err != nil {
		s.logger.Error("Failed to get comment replies", zap.String("comment_id", commentID), zap.Error(err))
		return nil, utils.NewInternalError("Failed to get replies", err)
	}

	// Enrich replies
	var enrichedReplies []*models.CommentResponse
	for _, reply := range replies {
		enrichedReply, err := s.enrichComment(ctx, reply, viewerID, false)
		if err != nil {
			s.logger.Warn("Failed to enrich reply", zap.String("comment_id", reply.ID), zap.Error(err))
			continue
		}
		enrichedReplies = append(enrichedReplies, enrichedReply)
	}

	return enrichedReplies, nil
}

// UpdateComment updates a comment
func (s *CommentService) UpdateComment(ctx context.Context, commentID, userID string, req *models.UpdateCommentRequest) (*models.CommentResponse, error) {
	// Get existing comment
	comment, err := s.commentRepo.GetByID(ctx, commentID)
	if err != nil {
		return nil, utils.NewNotFoundError("Comment not found", err)
	}

	// Check ownership
	if comment.UserID != userID {
		return nil, utils.NewUnauthorizedError("You don't have permission to update this comment", nil)
	}

	// Update comment
	comment.Text = req.Text
	comment.UpdatedAt = time.Now()

	if err := s.commentRepo.Update(ctx, comment); err != nil {
		s.logger.Error("Failed to update comment", zap.String("comment_id", commentID), zap.Error(err))
		return nil, utils.NewInternalError("Failed to update comment", err)
	}

	s.logger.Info("Comment updated", zap.String("comment_id", commentID), zap.String("user_id", userID))

	// Return enriched comment
	return s.GetComment(ctx, commentID, &userID)
}

// DeleteComment soft deletes a comment
func (s *CommentService) DeleteComment(ctx context.Context, commentID, userID string) error {
	// Get existing comment
	comment, err := s.commentRepo.GetByID(ctx, commentID)
	if err != nil {
		return utils.NewNotFoundError("Comment not found", err)
	}

	// Check ownership
	if comment.UserID != userID {
		return utils.NewUnauthorizedError("You don't have permission to delete this comment", nil)
	}

	// Delete comment
	if err := s.commentRepo.Delete(ctx, commentID); err != nil {
		s.logger.Error("Failed to delete comment", zap.String("comment_id", commentID), zap.Error(err))
		return utils.NewInternalError("Failed to delete comment", err)
	}

	s.logger.Info("Comment deleted", zap.String("comment_id", commentID), zap.String("user_id", userID))
	return nil
}

// LikeComment likes a comment
func (s *CommentService) LikeComment(ctx context.Context, userID, commentID string) error {
	// Check if comment exists
	if _, err := s.commentRepo.GetByID(ctx, commentID); err != nil {
		return utils.NewNotFoundError("Comment not found", err)
	}

	// Like comment (idempotent)
	if err := s.commentRepo.LikeComment(ctx, userID, commentID); err != nil {
		s.logger.Error("Failed to like comment", zap.String("comment_id", commentID), zap.Error(err))
		return utils.NewInternalError("Failed to like comment", err)
	}

	s.logger.Info("Comment liked", zap.String("comment_id", commentID), zap.String("user_id", userID))
	return nil
}

// UnlikeComment unlikes a comment
func (s *CommentService) UnlikeComment(ctx context.Context, userID, commentID string) error {
	// Unlike comment (idempotent)
	if err := s.commentRepo.UnlikeComment(ctx, userID, commentID); err != nil {
		s.logger.Error("Failed to unlike comment", zap.String("comment_id", commentID), zap.Error(err))
		return utils.NewInternalError("Failed to unlike comment", err)
	}

	s.logger.Info("Comment unliked", zap.String("comment_id", commentID), zap.String("user_id", userID))
	return nil
}

// enrichComment enriches a comment with author, attachments, and engagement status
func (s *CommentService) enrichComment(ctx context.Context, comment *models.PostComment, viewerID *string, includeReplies bool) (*models.CommentResponse, error) {
	response := &models.CommentResponse{
		ID:              comment.ID,
		PostID:          comment.PostID,
		Text:            comment.Text,
		ParentCommentID: comment.ParentCommentID,
		TotalLikes:      comment.TotalLikes,
		TotalReplies:    comment.TotalReplies,
		CreatedAt:       comment.CreatedAt,
		UpdatedAt:       comment.UpdatedAt,
	}

	// Get author info
	profile, err := s.userRepo.GetProfileByUserID(ctx, comment.UserID)
	if err == nil {
		response.Author = &models.AuthorInfo{
			UserID:       comment.UserID,
			FirstName:    profile.FirstName,
			LastName:     profile.LastName,
			FullName:     profile.FullName(),
			Avatar:       profile.Avatar,
			Province:     profile.Province,
			District:     profile.District,
			Neighborhood: profile.Neighborhood,
		}
	}

	// Get attachments
	attachments, err := s.commentRepo.GetAttachmentsByCommentID(ctx, comment.ID)
	if err == nil && len(attachments) > 0 {
		var photos []models.Photo
		for _, att := range attachments {
			photos = append(photos, att.Photo)
		}
		response.Attachments = photos
	}

	// Add location info
	if comment.Location != nil && comment.Location.Valid {
		response.Location = &models.LocationInfo{
			Latitude:  &comment.Location.P.Y,
			Longitude: &comment.Location.P.X,
		}
	}

	// Get engagement status if viewer is authenticated
	if viewerID != nil && *viewerID != "" {
		liked, err := s.commentRepo.IsLikedByUser(ctx, *viewerID, comment.ID)
		if err == nil {
			response.LikedByMe = liked
		}
	}

	// Get first few replies if requested
	if includeReplies && comment.TotalReplies > 0 {
		replies, err := s.GetCommentReplies(ctx, comment.ID, 3, 0, viewerID)
		if err == nil {
			response.Replies = replies
		}
	}

	return response, nil
}
