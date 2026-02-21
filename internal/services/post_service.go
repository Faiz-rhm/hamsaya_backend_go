package services

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/internal/repositories"
	"github.com/hamsaya/backend/internal/utils"
	"github.com/hamsaya/backend/pkg/storage"
	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"
)

// PostService handles post operations
type PostService struct {
	postRepo            repositories.PostRepository
	pollRepo            repositories.PollRepository
	userRepo            repositories.UserRepository
	businessRepo        repositories.BusinessRepository
	relationshipsRepo   repositories.RelationshipsRepository
	categoryRepo        repositories.CategoryRepository
	eventRepo           repositories.EventRepository
	notificationService *NotificationService
	storageBucketName   string
	logger              *zap.Logger
}

// NewPostService creates a new post service
func NewPostService(
	postRepo repositories.PostRepository,
	pollRepo repositories.PollRepository,
	userRepo repositories.UserRepository,
	businessRepo repositories.BusinessRepository,
	relationshipsRepo repositories.RelationshipsRepository,
	categoryRepo repositories.CategoryRepository,
	eventRepo repositories.EventRepository,
	notificationService *NotificationService,
	storageBucketName string,
	logger *zap.Logger,
) *PostService {
	return &PostService{
		postRepo:            postRepo,
		pollRepo:            pollRepo,
		userRepo:            userRepo,
		businessRepo:        businessRepo,
		relationshipsRepo:   relationshipsRepo,
		categoryRepo:        categoryRepo,
		eventRepo:           eventRepo,
		notificationService: notificationService,
		storageBucketName:   storageBucketName,
		logger:              logger,
	}
}

// CreatePost creates a new post
func (s *PostService) CreatePost(ctx context.Context, userID string, req *models.CreatePostRequest) (*models.PostResponse, error) {
	// Validate post type specific requirements
	if err := s.validatePostRequest(req); err != nil {
		return nil, err
	}

	// Create post
	postID := uuid.New().String()
	now := time.Now()

	post := &models.Post{
		ID:          postID,
		UserID:      &userID,
		BusinessID:  req.BusinessID,
		Type:        req.Type,
		Title:       req.Title,
		Description: req.Description,
		Status:      true,
		Visibility:  models.VisibilityPublic,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// Set visibility if provided
	if req.Visibility != "" {
		post.Visibility = req.Visibility
	}

	// Handle sell-specific fields
	if req.Type == models.PostTypeSell {
		post.Currency = req.Currency
		post.Price = req.Price
		post.Discount = req.Discount
		if req.Free != nil {
			post.Free = *req.Free
		}
		post.CategoryID = req.CategoryID
		post.CountryCode = req.CountryCode
		post.ContactNo = req.ContactNo

		// Auto-expire SELL posts after 30 days
		expiry := now.AddDate(0, 1, 0) // 1 month from creation
		post.ExpiredAt = &expiry
	}

	// Handle event-specific fields
	if req.Type == models.PostTypeEvent {
		post.StartDate = req.StartDate
		post.StartTime = req.StartTime
		post.EndDate = req.EndDate
		post.EndTime = req.EndTime
		eventState := models.EventStateUpcoming
		post.EventState = &eventState
	}

	// Handle location (top-level or nested from app) — must run before Create so DB has address_location/is_location
	lat, lon := req.Latitude, req.Longitude
	if (lat == nil || lon == nil) && req.Location != nil {
		lat, lon = req.Location.Latitude, req.Location.Longitude
	}
	if lat != nil && lon != nil {
		post.AddressLocation = &pgtype.Point{
			P:     pgtype.Vec2{X: *lon, Y: *lat},
			Valid: true,
		}
		post.Country = req.Country
		post.Province = req.Province
		post.District = req.District
		post.Neighborhood = req.Neighborhood
		if req.IsLocation != nil {
			post.IsLocation = *req.IsLocation
		} else {
			post.IsLocation = true
		}
	} else if req.IsLocation != nil {
		post.IsLocation = *req.IsLocation
	}

	// Shared post reference (must be set before Create)
	if req.OriginalPostID != nil {
		post.OriginalPostID = req.OriginalPostID
	}

	// Create post in database first (needed before creating poll)
	if err := s.postRepo.Create(ctx, post); err != nil {
		s.logger.Error("Failed to create post", zap.String("user_id", userID), zap.Error(err))
		return nil, utils.NewInternalError("Failed to create post", err)
	}

	// Handle poll creation for PULL posts
	if req.Type == models.PostTypePull {
		// Get poll options from either poll_options or poll.options
		var pollOptions []string
		if len(req.PollOptions) > 0 {
			pollOptions = req.PollOptions
		} else if req.Poll != nil && len(req.Poll.Options) > 0 {
			pollOptions = req.Poll.Options
		}

		if len(pollOptions) > 0 {
			// Create poll
			poll := &models.Poll{
				ID:        uuid.New().String(),
				PostID:    postID,
				CreatedAt: now,
				UpdatedAt: now,
			}

			if err := s.pollRepo.Create(ctx, poll); err != nil {
				s.logger.Error("Failed to create poll",
					zap.String("post_id", postID),
					zap.Error(err),
				)
				// Delete the post since poll creation failed
				_ = s.postRepo.Delete(ctx, postID)
				return nil, utils.NewInternalError("Failed to create poll for post", err)
			}

			// Create poll options
			for _, optionText := range pollOptions {
				option := &models.PollOption{
					ID:        uuid.New().String(),
					PollID:    poll.ID,
					Option:    optionText,
					VoteCount: 0,
					CreatedAt: now,
					UpdatedAt: now,
				}

				if err := s.pollRepo.CreateOption(ctx, option); err != nil {
					s.logger.Error("Failed to create poll option",
						zap.String("poll_id", poll.ID),
						zap.String("option", optionText),
						zap.Error(err),
					)
					// Continue with other options instead of failing completely
				}
			}

			s.logger.Info("Poll created for PULL post",
				zap.String("post_id", postID),
				zap.String("poll_id", poll.ID),
				zap.Int("options_count", len(pollOptions)),
			)
		}
	}

	// Create attachments if provided (full Photo or URL-only)
	if len(req.Attachments) > 0 {
		for _, raw := range req.Attachments {
			photo, err := models.ParseAttachmentPhoto(raw)
			if err != nil {
				s.logger.Warn("Failed to parse attachment", zap.Error(err))
				continue
			}
			if photo.URL == "" {
				continue
			}
			attachment := &models.Attachment{
				ID:        uuid.New().String(),
				PostID:    postID,
				Photo:     photo,
				CreatedAt: now,
				UpdatedAt: now,
			}

			if err := s.postRepo.CreateAttachment(ctx, attachment); err != nil {
				s.logger.Error("Failed to create attachment",
					zap.String("post_id", postID),
					zap.Error(err),
				)
			}
		}
	}

	s.logger.Info("Post created",
		zap.String("post_id", postID),
		zap.String("user_id", userID),
		zap.String("type", string(req.Type)),
	)

	// Notify followers of the new post (user followers or business followers)
	go s.notifyFollowersOfNewPost(context.WithoutCancel(ctx), postID, userID, req.BusinessID)

	// Return enriched post
	return s.GetPost(ctx, postID, &userID)
}

// GetPost gets a post by ID with full details
func (s *PostService) GetPost(ctx context.Context, postID string, viewerID *string) (*models.PostResponse, error) {
	// Get post
	post, err := s.postRepo.GetByID(ctx, postID)
	if err != nil {
		s.logger.Warn("Post not found", zap.String("post_id", postID), zap.Error(err))
		return nil, utils.NewNotFoundError("Post not found", err)
	}

	// Enrich post
	return s.enrichPost(ctx, post, viewerID)
}

// UpdatePost updates a post
func (s *PostService) UpdatePost(ctx context.Context, postID, userID string, req *models.UpdatePostRequest) (*models.PostResponse, error) {
	// Get existing post
	post, err := s.postRepo.GetByID(ctx, postID)
	if err != nil {
		return nil, utils.NewNotFoundError("Post not found", err)
	}

	// Check ownership
	if post.UserID == nil || *post.UserID != userID {
		return nil, utils.NewUnauthorizedError("You don't have permission to update this post", nil)
	}

	// Update fields
	if req.Title != nil {
		post.Title = req.Title
	}
	if req.Description != nil {
		post.Description = req.Description
	}
	if req.Visibility != nil {
		post.Visibility = *req.Visibility
	}
	if req.Price != nil {
		post.Price = req.Price
	}
	if req.Discount != nil {
		post.Discount = req.Discount
	}
	if req.Free != nil {
		post.Free = *req.Free
	}
	if req.Sold != nil {
		post.Sold = *req.Sold
	}
	if req.StartDate != nil {
		post.StartDate = req.StartDate
	}
	if req.StartTime != nil {
		post.StartTime = req.StartTime
	}
	if req.EndDate != nil {
		post.EndDate = req.EndDate
	}
	if req.EndTime != nil {
		post.EndTime = req.EndTime
	}
	if req.Currency != nil {
		post.Currency = req.Currency
	}
	if req.ContactNo != nil {
		post.ContactNo = req.ContactNo
	}
	if req.CountryCode != nil {
		post.CountryCode = req.CountryCode
	}
	if req.IsLocation != nil {
		post.IsLocation = *req.IsLocation
	}
	if req.CategoryID != nil {
		post.CategoryID = req.CategoryID
	}

	// Location: same logic as create (top-level or nested)
	lat, lon := req.Latitude, req.Longitude
	if (lat == nil || lon == nil) && req.Location != nil {
		lat, lon = req.Location.Latitude, req.Location.Longitude
	}
	if lat != nil && lon != nil {
		post.AddressLocation = &pgtype.Point{
			P:     pgtype.Vec2{X: *lon, Y: *lat},
			Valid: true,
		}
		if req.Country != nil {
			post.Country = req.Country
		}
		if req.Province != nil {
			post.Province = req.Province
		}
		if req.District != nil {
			post.District = req.District
		}
		if req.Neighborhood != nil {
			post.Neighborhood = req.Neighborhood
		}
		if req.IsLocation != nil {
			post.IsLocation = *req.IsLocation
		} else {
			post.IsLocation = true
		}
	}

	post.UpdatedAt = time.Now()

	// Update in database
	if err := s.postRepo.Update(ctx, post); err != nil {
		s.logger.Error("Failed to update post", zap.String("post_id", postID), zap.Error(err))
		return nil, utils.NewInternalError("Failed to update post", err)
	}

	// ── Attachment changes ──────────────────────────────────────────────

	// Remove requested attachments (scoped to this post for safety).
	for _, attID := range req.DeletedAttachments {
		if attID == "" {
			continue
		}
		if err := s.postRepo.DeleteAttachmentForPost(ctx, postID, attID); err != nil {
			s.logger.Warn("Failed to delete attachment on update",
				zap.String("post_id", postID),
				zap.String("attachment_id", attID),
				zap.Error(err),
			)
		}
	}

	// Add new attachments (same parsing as create: accepts Photo objects or bare URL strings).
	if len(req.Attachments) > 0 {
		now := time.Now()
		for _, raw := range req.Attachments {
			photo, err := models.ParseAttachmentPhoto(raw)
			if err != nil {
				s.logger.Warn("Failed to parse attachment on update", zap.Error(err))
				continue
			}
			if photo.URL == "" {
				continue
			}
			attachment := &models.Attachment{
				ID:        uuid.New().String(),
				PostID:    postID,
				Photo:     photo,
				CreatedAt: now,
				UpdatedAt: now,
			}
			if err := s.postRepo.CreateAttachment(ctx, attachment); err != nil {
				s.logger.Error("Failed to create attachment on update",
					zap.String("post_id", postID),
					zap.Error(err),
				)
			}
		}
	}

	// ── PULL: update poll options (replace all options when poll_options sent) ──
	isPull := strings.EqualFold(string(post.Type), string(models.PostTypePull))
	if isPull && len(req.PollOptions) >= 2 {
		s.logger.Info("Updating poll options for PULL post",
			zap.String("post_id", postID),
			zap.String("post_type", string(post.Type)),
			zap.Int("poll_options_count", len(req.PollOptions)),
			zap.Strings("poll_options", req.PollOptions),
		)
		poll, err := s.pollRepo.GetByPostID(ctx, postID)
		if err != nil {
			s.logger.Error("Failed to get poll for update", zap.String("post_id", postID), zap.Error(err))
			return nil, utils.NewNotFoundError("Poll not found for this post", err)
		}
		if err := s.pollRepo.DeleteOptionsByPollID(ctx, poll.ID); err != nil {
			s.logger.Error("Failed to delete old poll options", zap.String("poll_id", poll.ID), zap.Error(err))
			return nil, utils.NewInternalError("Failed to update poll options", err)
		}
		now := time.Now()
		for _, optionText := range req.PollOptions {
			option := &models.PollOption{
				ID:        uuid.New().String(),
				PollID:    poll.ID,
				Option:    optionText,
				VoteCount: 0,
				CreatedAt: now,
				UpdatedAt: now,
			}
			if err := s.pollRepo.CreateOption(ctx, option); err != nil {
				s.logger.Error("Failed to create poll option on update",
					zap.String("poll_id", poll.ID),
					zap.String("option", optionText),
					zap.Error(err),
				)
				return nil, utils.NewInternalError("Failed to create poll option: "+optionText, err)
			}
		}
		s.logger.Info("Poll options updated", zap.String("post_id", postID), zap.Int("options", len(req.PollOptions)))
	}

	s.logger.Info("Post updated", zap.String("post_id", postID), zap.String("user_id", userID))

	// Return enriched post
	return s.GetPost(ctx, postID, &userID)
}

// DeletePost soft deletes a post
func (s *PostService) DeletePost(ctx context.Context, postID, userID string) error {
	// Get existing post
	post, err := s.postRepo.GetByID(ctx, postID)
	if err != nil {
		return utils.NewNotFoundError("Post not found", err)
	}

	// Check ownership
	if post.UserID == nil || *post.UserID != userID {
		return utils.NewUnauthorizedError("You don't have permission to delete this post", nil)
	}

	// Delete post
	if err := s.postRepo.Delete(ctx, postID); err != nil {
		s.logger.Error("Failed to delete post", zap.String("post_id", postID), zap.Error(err))
		return utils.NewInternalError("Failed to delete post", err)
	}

	s.logger.Info("Post deleted", zap.String("post_id", postID), zap.String("user_id", userID))
	return nil
}

// LikePost likes a post
func (s *PostService) LikePost(ctx context.Context, userID, postID string) error {
	post, err := s.postRepo.GetByID(ctx, postID)
	if err != nil {
		return utils.NewNotFoundError("Post not found", err)
	}

	if err := s.postRepo.LikePost(ctx, userID, postID); err != nil {
		s.logger.Error("Failed to like post", zap.String("post_id", postID), zap.Error(err))
		return utils.NewInternalError("Failed to like post", err)
	}

	s.logger.Info("Post liked", zap.String("post_id", postID), zap.String("user_id", userID))

	if post.UserID != nil && *post.UserID != userID && s.notificationService != nil {
		go s.sendPostNotification(context.WithoutCancel(ctx), userID, *post.UserID, postID, models.NotificationTypeLike, "liked your post")
	}

	return nil
}

// UnlikePost unlikes a post
func (s *PostService) UnlikePost(ctx context.Context, userID, postID string) error {
	// Unlike post (idempotent)
	if err := s.postRepo.UnlikePost(ctx, userID, postID); err != nil {
		s.logger.Error("Failed to unlike post", zap.String("post_id", postID), zap.Error(err))
		return utils.NewInternalError("Failed to unlike post", err)
	}

	s.logger.Info("Post unliked", zap.String("post_id", postID), zap.String("user_id", userID))
	return nil
}

// BookmarkPost bookmarks a post
func (s *PostService) BookmarkPost(ctx context.Context, userID, postID string) error {
	// Check if post exists
	if _, err := s.postRepo.GetByID(ctx, postID); err != nil {
		return utils.NewNotFoundError("Post not found", err)
	}

	// Bookmark post (idempotent)
	if err := s.postRepo.BookmarkPost(ctx, userID, postID); err != nil {
		s.logger.Error("Failed to bookmark post", zap.String("post_id", postID), zap.Error(err))
		return utils.NewInternalError("Failed to bookmark post", err)
	}

	s.logger.Info("Post bookmarked", zap.String("post_id", postID), zap.String("user_id", userID))
	return nil
}

// UnbookmarkPost removes a bookmark
func (s *PostService) UnbookmarkPost(ctx context.Context, userID, postID string) error {
	// Unbookmark post (idempotent)
	if err := s.postRepo.UnbookmarkPost(ctx, userID, postID); err != nil {
		s.logger.Error("Failed to unbookmark post", zap.String("post_id", postID), zap.Error(err))
		return utils.NewInternalError("Failed to unbookmark post", err)
	}

	s.logger.Info("Post unbookmarked", zap.String("post_id", postID), zap.String("user_id", userID))
	return nil
}

// SharePost shares a post
func (s *PostService) SharePost(ctx context.Context, userID, originalPostID string, shareText *string) (*models.PostResponse, error) {
	// Check if original post exists
	originalPost, err := s.postRepo.GetByID(ctx, originalPostID)
	if err != nil {
		return nil, utils.NewNotFoundError("Original post not found", err)
	}

	// Create share record
	shareID := uuid.New().String()
	share := &models.PostShare{
		ID:             shareID,
		UserID:         userID,
		OriginalPostID: originalPostID,
		ShareText:      shareText,
		CreatedAt:      time.Now(),
	}

	// Always create a new post for the repost so it appears in the feed.
	// Whatever the original type (FEED, PULL, EVENT, SELL), the repost is always a FEED post.
	desc := "Shared a post"
	if shareText != nil && *shareText != "" {
		desc = *shareText
	}
	sharePostReq := &models.CreatePostRequest{
		Type:           models.PostTypeFeed,
		Description:    &desc,
		OriginalPostID: &originalPostID,
	}

	sharePost, err := s.CreatePost(ctx, userID, sharePostReq)
	if err != nil {
		return nil, err
	}

	share.SharedPostID = &sharePost.ID

	// Save share record
	if err := s.postRepo.SharePost(ctx, share); err != nil {
		s.logger.Error("Failed to share post", zap.String("post_id", originalPostID), zap.Error(err))
		return nil, utils.NewInternalError("Failed to share post", err)
	}

	s.logger.Info("Post shared",
		zap.String("original_post_id", originalPostID),
		zap.String("user_id", userID),
	)

	if originalPost.UserID != nil && *originalPost.UserID != userID && s.notificationService != nil {
		go s.sendPostNotification(context.WithoutCancel(ctx), userID, *originalPost.UserID, originalPostID, models.NotificationTypePostShare, "shared your post")
	}

	// Return the original post or the new shared post
	if share.SharedPostID != nil {
		return s.GetPost(ctx, *share.SharedPostID, &userID)
	}

	return s.GetPost(ctx, originalPostID, &userID)
}

// GetFeed gets posts for the feed
func (s *PostService) GetFeed(ctx context.Context, filter *models.FeedFilter, viewerID *string) ([]*models.PostResponse, int64, error) {
	// Get total count for pagination
	totalCount, err := s.postRepo.CountFeed(ctx, filter)
	if err != nil {
		s.logger.Error("Failed to count feed", zap.Error(err))
		return nil, 0, utils.NewInternalError("Failed to count feed", err)
	}

	// Get posts from repository
	posts, err := s.postRepo.GetFeed(ctx, filter)
	if err != nil {
		s.logger.Error("Failed to get feed", zap.Error(err))
		return nil, 0, utils.NewInternalError("Failed to get feed", err)
	}

	// Enrich posts
	var enrichedPosts []*models.PostResponse
	for _, post := range posts {
		enrichedPost, err := s.enrichPost(ctx, post, viewerID)
		if err != nil {
			s.logger.Warn("Failed to enrich post", zap.String("post_id", post.ID), zap.Error(err))
			continue
		}
		enrichedPosts = append(enrichedPosts, enrichedPost)
	}

	return enrichedPosts, totalCount, nil
}

// GetUserBookmarks gets bookmarked posts for a user
func (s *PostService) GetUserBookmarks(ctx context.Context, userID string, limit, offset int) ([]*models.PostResponse, error) {
	// Get bookmarked posts
	posts, err := s.postRepo.GetUserBookmarks(ctx, userID, limit, offset)
	if err != nil {
		s.logger.Error("Failed to get bookmarks", zap.String("user_id", userID), zap.Error(err))
		return nil, utils.NewInternalError("Failed to get bookmarks", err)
	}

	// Enrich posts
	var enrichedPosts []*models.PostResponse
	for _, post := range posts {
		enrichedPost, err := s.enrichPost(ctx, post, &userID)
		if err != nil {
			s.logger.Warn("Failed to enrich post", zap.String("post_id", post.ID), zap.Error(err))
			continue
		}
		enrichedPosts = append(enrichedPosts, enrichedPost)
	}

	return enrichedPosts, nil
}

// enrichPost enriches a post with author, attachments, and engagement status
func (s *PostService) enrichPost(ctx context.Context, post *models.Post, viewerID *string) (*models.PostResponse, error) {
	response := &models.PostResponse{
		ID:            post.ID,
		Type:          post.Type,
		Title:         post.Title,
		Description:   post.Description,
		Visibility:    post.Visibility,
		Status:        post.Status,
		BusinessID:    post.BusinessID,
		TotalComments: post.TotalComments,
		TotalLikes:    post.TotalLikes,
		TotalShares:   post.TotalShares,
		CreatedAt:     post.CreatedAt,
		UpdatedAt:     post.UpdatedAt,
	}

	// Get author info
	if post.UserID != nil {
		profile, err := s.userRepo.GetProfileByUserID(ctx, *post.UserID)
		if err == nil {
			response.Author = &models.AuthorInfo{
				UserID:       *post.UserID,
				FirstName:    profile.FirstName,
				LastName:     profile.LastName,
				FullName:     profile.FullName(),
				Avatar:       profile.Avatar,
				Province:     profile.Province,
				District:     profile.District,
				Neighborhood: profile.Neighborhood,
			}
		}
	}

	// Get business info if post is from a business
	var fetchedBusiness *models.BusinessProfile
	if post.BusinessID != nil && *post.BusinessID != "" {
		business, err := s.businessRepo.GetByID(ctx, *post.BusinessID)
		if err == nil {
			fetchedBusiness = business
			response.Business = &models.BusinessInfo{
				BusinessID:   business.ID,
				Name:         business.Name,
				Description:  business.Description,
				PhoneNumber:  business.PhoneNumber,
				Email:        business.Email,
				Website:      business.Website,
				Avatar:       business.Avatar,
				Cover:        business.Cover,
				Province:     business.Province,
				District:     business.District,
				Neighborhood: business.Neighborhood,
			}
		}
	}

	// Get attachments (return full objects with IDs so the client can reference them)
	attachments, err := s.postRepo.GetAttachmentsByPostID(ctx, post.ID)
	if err == nil && len(attachments) > 0 {
		bucket := s.storageBucketName
		if bucket == "" {
			bucket = "hamsaya-uploads"
		}
		for _, att := range attachments {
			photo := att.Photo
			photo.URL = storage.EnsureBucketInStorageURL(photo.URL, bucket)
			response.Attachments = append(response.Attachments, models.AttachmentResponse{
				ID:    att.ID,
				Photo: photo,
			})
		}
	}

	// Add type-specific fields
	if post.Type == models.PostTypeSell {
		response.Currency = post.Currency
		response.Price = post.Price
		response.Discount = post.Discount
		response.Free = &post.Free
		response.Sold = &post.Sold
		response.IsPromoted = &post.IsPromoted
		response.ContactNo = post.ContactNo
		response.IsLocation = &post.IsLocation
		response.ExpiredAt = post.ExpiredAt

		// Get category info if post has a category
		if post.CategoryID != nil && *post.CategoryID != "" {
			response.CategoryID = post.CategoryID
			category, err := s.categoryRepo.GetByID(ctx, *post.CategoryID)
			if err == nil {
				response.Category = &models.CategoryInfo{
					ID:    category.ID,
					Name:  category.Name,
					Icon:  models.Icon{Name: category.Icon.Name, Library: category.Icon.Library},
					Color: category.Color,
				}
			}
		}
	}

	if post.Type == models.PostTypeEvent {
		response.StartDate = post.StartDate
		response.StartTime = post.StartTime
		response.EndDate = post.EndDate
		response.EndTime = post.EndTime
		response.EventState = post.EventState
		response.InterestedCount = &post.InterestedCount
		response.GoingCount = &post.GoingCount
		// Current user's event interest (interested/going/not_interested)
		if viewerID != nil && *viewerID != "" {
			if userInterest, err := s.eventRepo.GetUserInterest(ctx, *viewerID, post.ID); err == nil && userInterest != nil {
				response.UserEventState = &userInterest.EventState
			}
		}
	}

	// Add location info (is_location for both SELL and EVENT so discover/map work)
	if post.AddressLocation != nil && post.AddressLocation.Valid {
		response.Location = &models.LocationInfo{
			Latitude:     &post.AddressLocation.P.Y,
			Longitude:    &post.AddressLocation.P.X,
			Country:      post.Country,
			Province:     post.Province,
			District:     post.District,
			Neighborhood: post.Neighborhood,
		}
	}
	response.IsLocation = &post.IsLocation

	// Get engagement status if viewer is authenticated
	if viewerID != nil && *viewerID != "" {
		liked, bookmarked, err := s.postRepo.GetEngagementStatus(ctx, *viewerID, post.ID)
		if err == nil {
			response.LikedByMe = liked
			response.BookmarkedByMe = bookmarked
		}

		// Check if post belongs to viewer (direct or via business ownership)
		if post.UserID != nil && *post.UserID == *viewerID {
			response.IsMine = true
		} else if fetchedBusiness != nil && fetchedBusiness.UserID == *viewerID {
			response.IsMine = true
		}
	}

	// Get original post if this is a share (only 1 level deep to prevent infinite recursion)
	if post.OriginalPostID != nil && *post.OriginalPostID != "" {
		originalPost, err := s.postRepo.GetByID(ctx, *post.OriginalPostID)
		if err == nil {
			// Enrich the original post, but pass nil for depth to avoid nested original posts
			enrichedOriginal, err := s.enrichPostSimple(ctx, originalPost, viewerID)
			if err == nil {
				response.OriginalPost = enrichedOriginal
			}
		}
	}

	return response, nil
}

// enrichPostSimple enriches a post with basic info without loading nested original posts
// Used for preventing infinite recursion when enriching shared posts
func (s *PostService) enrichPostSimple(ctx context.Context, post *models.Post, viewerID *string) (*models.PostResponse, error) {
	response := &models.PostResponse{
		ID:            post.ID,
		Type:          post.Type,
		Title:         post.Title,
		Description:   post.Description,
		Visibility:    post.Visibility,
		Status:        post.Status,
		BusinessID:    post.BusinessID,
		TotalComments: post.TotalComments,
		TotalLikes:    post.TotalLikes,
		TotalShares:   post.TotalShares,
		CreatedAt:     post.CreatedAt,
		UpdatedAt:     post.UpdatedAt,
	}

	// Get author info
	if post.UserID != nil {
		profile, err := s.userRepo.GetProfileByUserID(ctx, *post.UserID)
		if err == nil {
			response.Author = &models.AuthorInfo{
				UserID:       *post.UserID,
				FirstName:    profile.FirstName,
				LastName:     profile.LastName,
				FullName:     profile.FullName(),
				Avatar:       profile.Avatar,
				Province:     profile.Province,
				District:     profile.District,
				Neighborhood: profile.Neighborhood,
			}
		}
	}

	// Get business info if post is from a business
	var fetchedBusinessSimple *models.BusinessProfile
	if post.BusinessID != nil && *post.BusinessID != "" {
		business, err := s.businessRepo.GetByID(ctx, *post.BusinessID)
		if err == nil {
			fetchedBusinessSimple = business
			response.Business = &models.BusinessInfo{
				BusinessID:   business.ID,
				Name:         business.Name,
				Description:  business.Description,
				PhoneNumber:  business.PhoneNumber,
				Email:        business.Email,
				Website:      business.Website,
				Avatar:       business.Avatar,
				Cover:        business.Cover,
				Province:     business.Province,
				District:     business.District,
				Neighborhood: business.Neighborhood,
			}
		}
	}

	// Get attachments (return full objects with IDs)
	attachments, err := s.postRepo.GetAttachmentsByPostID(ctx, post.ID)
	if err == nil && len(attachments) > 0 {
		bucket := s.storageBucketName
		if bucket == "" {
			bucket = "hamsaya-uploads"
		}
		for _, att := range attachments {
			photo := att.Photo
			photo.URL = storage.EnsureBucketInStorageURL(photo.URL, bucket)
			response.Attachments = append(response.Attachments, models.AttachmentResponse{
				ID:    att.ID,
				Photo: photo,
			})
		}
	}

	// Add type-specific fields
	if post.Type == models.PostTypeSell {
		response.Currency = post.Currency
		response.Price = post.Price
		response.Discount = post.Discount
		response.Free = &post.Free
		response.Sold = &post.Sold
		response.IsPromoted = &post.IsPromoted
		response.ContactNo = post.ContactNo
		response.IsLocation = &post.IsLocation
		response.ExpiredAt = post.ExpiredAt

		// Get category info if post has a category
		if post.CategoryID != nil && *post.CategoryID != "" {
			response.CategoryID = post.CategoryID
			category, err := s.categoryRepo.GetByID(ctx, *post.CategoryID)
			if err == nil {
				response.Category = &models.CategoryInfo{
					ID:    category.ID,
					Name:  category.Name,
					Icon:  models.Icon{Name: category.Icon.Name, Library: category.Icon.Library},
					Color: category.Color,
				}
			}
		}
	}

	if post.Type == models.PostTypeEvent {
		response.StartDate = post.StartDate
		response.StartTime = post.StartTime
		response.EndDate = post.EndDate
		response.EndTime = post.EndTime
		response.EventState = post.EventState
		response.InterestedCount = &post.InterestedCount
		response.GoingCount = &post.GoingCount
		// Current user's event interest (interested/going/not_interested)
		if viewerID != nil && *viewerID != "" {
			if userInterest, err := s.eventRepo.GetUserInterest(ctx, *viewerID, post.ID); err == nil && userInterest != nil {
				response.UserEventState = &userInterest.EventState
			}
		}
	}

	// Add location info (is_location for both SELL and EVENT so discover/map work)
	if post.AddressLocation != nil && post.AddressLocation.Valid {
		response.Location = &models.LocationInfo{
			Latitude:     &post.AddressLocation.P.Y,
			Longitude:    &post.AddressLocation.P.X,
			Country:      post.Country,
			Province:     post.Province,
			District:     post.District,
			Neighborhood: post.Neighborhood,
		}
	}
	response.IsLocation = &post.IsLocation

	// Get engagement status if viewer is authenticated
	if viewerID != nil && *viewerID != "" {
		liked, bookmarked, err := s.postRepo.GetEngagementStatus(ctx, *viewerID, post.ID)
		if err == nil {
			response.LikedByMe = liked
			response.BookmarkedByMe = bookmarked
		}

		// Check if post belongs to viewer (direct or via business ownership)
		if post.UserID != nil && *post.UserID == *viewerID {
			response.IsMine = true
		} else if fetchedBusinessSimple != nil && fetchedBusinessSimple.UserID == *viewerID {
			response.IsMine = true
		}
	}

	// Note: OriginalPost is NOT enriched here to prevent infinite recursion

	return response, nil
}

// sendPostNotification fires a notification for the post owner when someone likes or shares the post.
// If the post belongs to a business, data.business_id is set so it only appears in business notifications.
func (s *PostService) sendPostNotification(ctx context.Context, actorUserID, recipientUserID, postID string, notifType models.NotificationType, action string) {
	actorName := "Someone"
	var actorAvatar interface{}
	if actor, err := s.userRepo.GetProfileByUserID(ctx, actorUserID); err == nil {
		actorName = actor.FullName()
		actorAvatar = actor.Avatar
	} else {
		s.logger.Warn("Failed to get actor profile for notification, using fallback", zap.Error(err), zap.String("actor_user_id", actorUserID))
	}
	title := actorName + " " + action
	msg := title
	data := map[string]interface{}{
		"actor_id":     actorUserID,
		"actor_name":   actorName,
		"actor_avatar": actorAvatar,
		"post_id":      postID,
	}
	if post, err := s.postRepo.GetByID(ctx, postID); err == nil && post.BusinessID != nil && *post.BusinessID != "" {
		data["business_id"] = *post.BusinessID
	}
	s.notificationService.CreateNotification(ctx, &models.CreateNotificationRequest{
		UserID:  recipientUserID,
		Type:    notifType,
		Title:   &title,
		Message: &msg,
		Data:    data,
	})
}

const _newPostNotifyBatchSize = 300

// notifyFollowersOfNewPost notifies all followers of the user or business when a new post is created.
func (s *PostService) notifyFollowersOfNewPost(ctx context.Context, postID, posterUserID string, businessID *string) {
	defer func() {
		if r := recover(); r != nil {
			s.logger.Error("notifyFollowersOfNewPost panic", zap.Any("panic", r), zap.String("post_id", postID))
		}
	}()

	businessIDVal := ""
	if businessID != nil {
		businessIDVal = *businessID
	}
	s.logger.Info("New post: notifying followers",
		zap.String("post_id", postID),
		zap.String("poster_user_id", posterUserID),
		zap.String("business_id", businessIDVal))

	if s.notificationService == nil {
		s.logger.Warn("notificationService is nil, skipping new post notifications")
		return
	}

	actorName := "Someone"
	var actorAvatar interface{}
	if actor, err := s.userRepo.GetProfileByUserID(ctx, posterUserID); err == nil {
		actorName = actor.FullName()
		actorAvatar = actor.Avatar
	}

	displayName := actorName
	if businessID != nil && *businessID != "" {
		if biz, err := s.businessRepo.GetByID(ctx, *businessID); err == nil && biz.Name != "" {
			displayName = biz.Name
		}
	}

	title := displayName + " posted"
	msg := title
	data := map[string]interface{}{
		"actor_id":     posterUserID,
		"actor_name":   displayName,
		"actor_avatar": actorAvatar,
		"post_id":      postID,
	}
	if businessID != nil && *businessID != "" {
		data["business_id"] = *businessID
	}

	var followerIDs []string
	if businessID != nil && *businessID != "" {
		offset := 0
		for {
			ids, err := s.businessRepo.GetFollowers(ctx, *businessID, _newPostNotifyBatchSize, offset)
			if err != nil {
				s.logger.Warn("GetFollowers (business) failed", zap.String("business_id", *businessID), zap.Error(err))
				break
			}
			if len(ids) == 0 {
				break
			}
			followerIDs = append(followerIDs, ids...)
			if len(ids) < _newPostNotifyBatchSize {
				break
			}
			offset += _newPostNotifyBatchSize
		}
		s.logger.Info("New post: business followers loaded",
			zap.String("post_id", postID),
			zap.String("business_id", *businessID),
			zap.Int("count", len(followerIDs)))
	} else {
		offset := 0
		for {
			follows, err := s.relationshipsRepo.GetFollowers(ctx, posterUserID, _newPostNotifyBatchSize, offset)
			if err != nil {
				s.logger.Warn("GetFollowers (user) failed", zap.String("user_id", posterUserID), zap.Error(err))
				break
			}
			if len(follows) == 0 {
				break
			}
			for _, f := range follows {
				followerIDs = append(followerIDs, f.FollowerID)
			}
			if len(follows) < _newPostNotifyBatchSize {
				break
			}
			offset += _newPostNotifyBatchSize
		}
		s.logger.Info("New post: user followers loaded",
			zap.String("post_id", postID),
			zap.String("poster_user_id", posterUserID),
			zap.Int("count", len(followerIDs)))
	}

	sent := 0
	for _, recipientID := range followerIDs {
		if recipientID == posterUserID {
			continue
		}
		_, err := s.notificationService.CreateNotification(ctx, &models.CreateNotificationRequest{
			UserID:  recipientID,
			Type:    models.NotificationTypeNewPost,
			Title:   &title,
			Message: &msg,
			Data:    data,
		})
		if err != nil {
			s.logger.Warn("CreateNotification (NEW_POST) failed", zap.String("recipient_id", recipientID), zap.Error(err))
			continue
		}
		sent++
	}
	s.logger.Info("New post: notifications sent", zap.String("post_id", postID), zap.Int("sent", sent))
}

// validatePostRequest validates post creation request
func (s *PostService) validatePostRequest(req *models.CreatePostRequest) error {
	switch req.Type {
	case models.PostTypeSell:
		if req.Title == nil || *req.Title == "" {
			return utils.NewBadRequestError("Title is required for sell posts", nil)
		}
		if req.Price == nil && (req.Free == nil || !*req.Free) {
			return utils.NewBadRequestError("Price is required for sell posts (or mark as free)", nil)
		}
	case models.PostTypeEvent:
		if req.Title == nil || *req.Title == "" {
			return utils.NewBadRequestError("Title is required for event posts", nil)
		}
		if req.StartDate == nil {
			return utils.NewBadRequestError("Start date is required for event posts", nil)
		}
	case models.PostTypePull:
		if req.Description == nil || *req.Description == "" {
			return utils.NewBadRequestError("Description is required for pull posts", nil)
		}
		// Check both poll formats (poll_options or poll.options)
		pollOptionsCount := len(req.PollOptions)
		if req.Poll != nil {
			pollOptionsCount = len(req.Poll.Options)
		}
		if pollOptionsCount < 2 {
			return utils.NewBadRequestError("Poll options are required for pull posts (minimum 2 options)", nil)
		}
		if pollOptionsCount > 10 {
			return utils.NewBadRequestError("Maximum 10 poll options allowed", nil)
		}
	case models.PostTypeFeed:
		if req.Description == nil || *req.Description == "" {
			return utils.NewBadRequestError("Description is required for feed posts", nil)
		}
	}

	return nil
}
