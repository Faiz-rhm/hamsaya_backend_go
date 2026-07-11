package services

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/internal/repositories"
	"github.com/hamsaya/backend/internal/utils"
	"github.com/hamsaya/backend/pkg/cache"
	"go.uber.org/zap"
)

// discoverTTL is the cache lifetime for the geographic discover response.
// 30 seconds is long enough that one user moving the radius slider or
// reopening the tab pulls from cache; short enough that a freshly-created
// post / business / event surfaces almost immediately. The (lat, lng)
// inputs are bucketed to ~110m so users in the same neighbourhood share
// cache entries even when their GPS jitters.
const discoverTTL = 30 * time.Second

// SearchService handles search and discovery operations
type SearchService struct {
	searchRepo        repositories.SearchRepository
	postRepo          repositories.PostRepository
	userRepo          repositories.UserRepository
	businessRepo      repositories.BusinessRepository
	categoryRepo      repositories.CategoryRepository
	relationshipsRepo repositories.RelationshipsRepository
	logger            *zap.Logger
	cache             *cache.Cache // optional; nil = no discover caching
}

// NewSearchService creates a new search service
func NewSearchService(
	searchRepo repositories.SearchRepository,
	postRepo repositories.PostRepository,
	userRepo repositories.UserRepository,
	businessRepo repositories.BusinessRepository,
	categoryRepo repositories.CategoryRepository,
	relationshipsRepo repositories.RelationshipsRepository,
	logger *zap.Logger,
) *SearchService {
	return &SearchService{
		searchRepo:        searchRepo,
		postRepo:          postRepo,
		userRepo:          userRepo,
		businessRepo:      businessRepo,
		categoryRepo:      categoryRepo,
		relationshipsRepo: relationshipsRepo,
		logger:            logger,
	}
}

// WithCache attaches a cache namespace. Call once at startup. Optional.
func (s *SearchService) WithCache(c *cache.Cache) *SearchService {
	s.cache = c
	return s
}

// discoverCacheKey buckets the (lat, lng) inputs to 3 decimal places —
// roughly 110m at the equator, smaller toward the poles — so callers in
// the same neighbourhood hit the same cache entry. Other inputs (filter,
// type, radius, limit) participate verbatim because their cardinality is
// small and they radically change the result set.
func discoverCacheKey(req *models.DiscoverRequest) string {
	bucket := func(f float64) float64 {
		return math.Round(f*1000) / 1000
	}
	pt := "any"
	if req.Type != nil {
		pt = string(*req.Type)
	}
	return fmt.Sprintf("d:%s:%s:%.3f:%.3f:%.0f:%d",
		req.Filter, pt, bucket(req.Latitude), bucket(req.Longitude), req.RadiusKm, req.Limit)
}

// Search performs a global search across posts, users, and businesses
func (s *SearchService) Search(ctx context.Context, userID *string, req *models.SearchRequest) (*models.SearchResponse, error) {
	filter := &models.SearchFilter{
		Query:     req.Query,
		Type:      req.Type,
		Limit:     req.Limit,
		Offset:    req.Offset,
		UserID:    userID,
		Latitude:  req.Latitude,
		Longitude: req.Longitude,
		RadiusKm:  req.RadiusKm,
	}

	// Set default limit
	if filter.Limit == 0 {
		filter.Limit = 20
	}

	response := &models.SearchResponse{
		Posts:      []*models.PostResponse{},
		Users:      []*models.UserSearchResult{},
		Businesses: []*models.BusinessSearchResult{},
	}

	// Search based on type
	switch filter.Type {
	case models.SearchTypePosts:
		posts, err := s.searchRepo.SearchPosts(ctx, filter)
		if err != nil {
			return nil, utils.NewInternalError("Failed to search posts", err)
		}
		response.Posts = s.enrichPosts(ctx, posts, userID)
		response.Total = len(response.Posts)

	case models.SearchTypeUsers:
		profiles, err := s.searchRepo.SearchUsers(ctx, filter)
		if err != nil {
			return nil, utils.NewInternalError("Failed to search users", err)
		}
		response.Users = s.enrichUsers(ctx, profiles, userID)
		response.Total = len(response.Users)

	case models.SearchTypeBusinesses:
		businesses, err := s.searchRepo.SearchBusinesses(ctx, filter)
		if err != nil {
			return nil, utils.NewInternalError("Failed to search businesses", err)
		}
		response.Businesses = s.enrichBusinesses(ctx, businesses, userID)
		response.Total = len(response.Businesses)

	case models.SearchTypeAll, "":
		// Search all types with smaller limits per type
		filter.Limit = 10

		posts, _ := s.searchRepo.SearchPosts(ctx, filter)
		response.Posts = s.enrichPosts(ctx, posts, userID)

		profiles, _ := s.searchRepo.SearchUsers(ctx, filter)
		response.Users = s.enrichUsers(ctx, profiles, userID)

		businesses, _ := s.searchRepo.SearchBusinesses(ctx, filter)
		response.Businesses = s.enrichBusinesses(ctx, businesses, userID)

		response.Total = len(response.Posts) + len(response.Users) + len(response.Businesses)
	}

	s.logger.Info("Search completed",
		zap.String("query", req.Query),
		zap.String("type", string(req.Type)),
		zap.Int("total_results", response.Total),
	)

	return response, nil
}

// Discover performs location-based discovery for map view.
// Filter: all = posts (EVENT+SELL) + businesses; business = only businesses; event = only EVENT posts; sell = only SELL posts.
func (s *SearchService) Discover(ctx context.Context, userID *string, req *models.DiscoverRequest) (*models.DiscoverResponse, error) {
	// Set default limit
	limit := req.Limit
	if limit == 0 {
		limit = 100
	}
	req.Limit = limit

	// Cache lookup — discover response is intentionally viewer-agnostic
	// (no liked-by-me / following fields in the markers), so all viewers
	// in the same geographic bucket share a single cache entry. Massive
	// hit rate on the Discover tab cold-open and radius-slider drag.
	cacheKey := discoverCacheKey(req)
	if s.cache != nil {
		var cached models.DiscoverResponse
		if hit, _ := s.cache.Get(ctx, cacheKey, &cached); hit {
			return &cached, nil
		}
	}

	response := &models.DiscoverResponse{
		Posts:      []*models.DiscoverPost{},
		Businesses: []*models.DiscoverBusiness{},
	}

	filter := req.Filter
	if filter == "" {
		filter = models.DiscoverFilterAll
	}

	// Fetch posts only when filter is all, event, or sell
	if filter == models.DiscoverFilterAll || filter == models.DiscoverFilterEvent || filter == models.DiscoverFilterSell {
		var postType *models.PostType
		switch filter {
		case models.DiscoverFilterEvent:
			pt := models.PostTypeEvent
			postType = &pt
		case models.DiscoverFilterSell:
			pt := models.PostTypeSell
			postType = &pt
		}
		// if all, postType stays nil so GetDiscoverPosts returns both EVENT and SELL
		posts, err := s.searchRepo.GetDiscoverPosts(ctx, req.Latitude, req.Longitude, req.RadiusKm, postType, limit)
		if err != nil {
			s.logger.Error("Failed to get discover posts", zap.Error(err))
		} else {
			response.Posts = s.enrichDiscoverPosts(ctx, posts, userID == nil || *userID == "")
		}
	}

	// Fetch businesses only when filter is all or business
	if filter == models.DiscoverFilterAll || filter == models.DiscoverFilterBusiness {
		businesses, err := s.searchRepo.GetDiscoverBusinesses(ctx, req.Latitude, req.Longitude, req.RadiusKm, limit)
		if err != nil {
			s.logger.Error("Failed to get discover businesses", zap.Error(err))
		} else {
			response.Businesses = s.enrichDiscoverBusinesses(ctx, businesses, userID == nil || *userID == "")
		}
	}

	response.Total = len(response.Posts) + len(response.Businesses)

	s.logger.Info("Discovery completed",
		zap.Float64("latitude", req.Latitude),
		zap.Float64("longitude", req.Longitude),
		zap.Float64("radius_km", req.RadiusKm),
		zap.Int("total_results", response.Total),
	)

	if s.cache != nil {
		_ = s.cache.Set(ctx, cacheKey, response, discoverTTL)
	}

	return response, nil
}

// enrichPosts enriches post search results
func (s *SearchService) enrichPosts(ctx context.Context, posts []*models.Post, userID *string) []*models.PostResponse {
	var responses []*models.PostResponse

	for _, post := range posts {
		// Get author info
		var author *models.AuthorInfo
		if post.UserID != nil && *post.UserID != "" {
			if profile, err := s.userRepo.GetProfileByUserID(ctx, *post.UserID); err == nil {
				avatarColor := profile.AvatarColor
				if avatarColor == nil || *avatarColor == "" {
					c := models.DefaultAvatarColorForProfile(profile.ID)
					avatarColor = &c
				}
				author = &models.AuthorInfo{
					UserID:       *post.UserID,
					FirstName:    profile.FirstName,
					LastName:     profile.LastName,
					FullName:     profile.FullName(),
					Avatar:       profile.Avatar,
					AvatarColor:  avatarColor,
					Province:     profile.Province,
					District:     profile.District,
					Neighborhood: profile.Neighborhood,
				}
			}
		}

		// Check if liked/bookmarked by current user
		var likedByMe, bookmarkedByMe, isMine bool
		if userID != nil {
			likedByMe, _ = s.postRepo.IsLikedByUser(ctx, post.ID, *userID)
			bookmarkedByMe, _ = s.postRepo.IsBookmarkedByUser(ctx, post.ID, *userID)

			// Check if post belongs to viewer
			if post.UserID != nil && *post.UserID == *userID {
				isMine = true
			} else if post.BusinessID != nil && *post.BusinessID == *userID {
				isMine = true
			}
		}

		// Convert Free and Sold to pointers
		free := post.Free
		sold := post.Sold

		response := &models.PostResponse{
			ID:             post.ID,
			Author:         author,
			Type:           post.Type,
			Title:          post.Title,
			Description:    post.Description,
			Visibility:     post.Visibility,
			Status:         post.Status,
			Attachments:    []models.AttachmentResponse{}, // Will be populated if needed
			TotalComments:  post.TotalComments,
			TotalLikes:     post.TotalLikes,
			TotalShares:    post.TotalShares,
			LikedByMe:      likedByMe,
			BookmarkedByMe: bookmarkedByMe,
			IsMine:         isMine,
			CreatedAt:      post.CreatedAt,
			UpdatedAt:      post.UpdatedAt,
		}

		// Add type-specific fields
		if post.Type == models.PostTypeSell {
			response.Price = post.Price
			response.Currency = post.Currency
			response.Discount = post.Discount
			response.Free = &free
			response.Sold = &sold

			// Get category info
			if post.CategoryID != nil && *post.CategoryID != "" {
				if category, err := s.categoryRepo.GetByID(ctx, *post.CategoryID); err == nil {
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
			response.InterestedCount = &post.InterestedCount
			response.GoingCount = &post.GoingCount
		}

		responses = append(responses, response)
	}

	return responses
}

// enrichUsers enriches user search results
func (s *SearchService) enrichUsers(ctx context.Context, profiles []*models.Profile, userID *string) []*models.UserSearchResult {
	var results []*models.UserSearchResult

	for _, profile := range profiles {
		// Convert profile to UserSearchResult
		result := models.ToUserSearchResult(profile)

		// Check relationship status
		if userID != nil {
			result.IsFollowing, _ = s.relationshipsRepo.IsFollowing(ctx, *userID, profile.ID)
			result.IsFollowedBy, _ = s.relationshipsRepo.IsFollowing(ctx, profile.ID, *userID)
		}

		results = append(results, result)
	}

	return results
}

// enrichBusinesses enriches business search results
func (s *SearchService) enrichBusinesses(ctx context.Context, businesses []*models.BusinessProfile, userID *string) []*models.BusinessSearchResult {
	var results []*models.BusinessSearchResult

	for _, business := range businesses {
		// Extract location
		var location *models.Location
		if business.AddressLocation != nil {
			location = &models.Location{
				Country:  business.Country,
				Province: business.Province,
				District: business.District,
			}
		}

		result := &models.BusinessSearchResult{
			ID:          business.ID,
			Name:        business.Name,
			Description: business.Description,
			Avatar:      business.Avatar,
			Cover:       business.Cover,
			Address:     business.Address,
			PhoneNumber: business.PhoneNumber,
			Website:     business.Website,
			Categories:  []string{}, // Categories will be empty for now
			Location:    location,
			TotalFollow: business.TotalFollow,
			TotalViews:  business.TotalViews,
			IsFollowing: false, // Can be enriched if needed
		}

		results = append(results, result)
	}

	return results
}

// enrichDiscoverPosts enriches discover post results. Fetches first
// attachment per post in a single batched query so the mobile client doesn't
// have to issue one /posts/{id}/attachments request per marker card.
func (s *SearchService) enrichDiscoverPosts(ctx context.Context, posts []*models.Post, anon bool) []*models.DiscoverPost {
	results := make([]*models.DiscoverPost, 0, len(posts))

	// Batched fetch of first attachment per post.
	postIDs := make([]string, 0, len(posts))
	for _, p := range posts {
		postIDs = append(postIDs, p.ID)
	}
	attachmentsByPost := map[string][]*models.Attachment{}
	if len(postIDs) > 0 {
		if a, err := s.postRepo.GetAttachmentsByPostIDs(ctx, postIDs); err == nil {
			attachmentsByPost = a
		} else {
			s.logger.Warn("Failed to batch-load post attachments for discover", zap.Error(err))
		}
	}

	for _, post := range posts {
		var location *models.Location
		if post.AddressLocation != nil && post.AddressLocation.Valid {
			lat, lng := post.AddressLocation.P.Y, post.AddressLocation.P.X
			if anon {
				// Anonymous map browsing gets area-level pins (~1 km), not
				// exact seller/home coordinates.
				lat = math.Round(lat*100) / 100
				lng = math.Round(lng*100) / 100
			}
			location = &models.Location{
				Latitude:  lat,
				Longitude: lng,
				Country:   post.Country,
				Province:  post.Province,
				District:  post.District,
			}
		}

		var startDate, startTime *string
		if post.StartDate != nil {
			dateStr := post.StartDate.Format("2006-01-02")
			startDate = &dateStr
		}
		if post.StartTime != nil {
			timeStr := post.StartTime.Format("15:04")
			startTime = &timeStr
		}

		var thumbnail *models.Photo
		if attachments := attachmentsByPost[post.ID]; len(attachments) > 0 {
			photo := attachments[0].Photo
			if photo.URL != "" {
				thumbnail = &photo
			}
		}

		result := &models.DiscoverPost{
			ID:          post.ID,
			Type:        post.Type,
			Title:       post.Title,
			Description: post.Description,
			Thumbnail:   thumbnail,
			Location:    location,
			Price:       post.Price,
			StartDate:   startDate,
			StartTime:   startTime,
			CreatedAt:   post.CreatedAt,
		}

		results = append(results, result)
	}

	return results
}

// enrichDiscoverBusinesses enriches discover business results. Fetches
// category-name lists for all businesses in a single batched query so the
// mobile client doesn't have to issue one /businesses/{id}/categories
// request per marker card.
func (s *SearchService) enrichDiscoverBusinesses(ctx context.Context, businesses []*models.BusinessProfile, anon bool) []*models.DiscoverBusiness {
	results := make([]*models.DiscoverBusiness, 0, len(businesses))

	businessIDs := make([]string, 0, len(businesses))
	for _, b := range businesses {
		businessIDs = append(businessIDs, b.ID)
	}
	categoriesByBusiness := map[string][]string{}
	if len(businessIDs) > 0 {
		if c, err := s.businessRepo.GetCategoriesByBusinessIDs(ctx, businessIDs); err == nil {
			categoriesByBusiness = c
		} else {
			s.logger.Warn("Failed to batch-load business categories for discover", zap.Error(err))
		}
	}

	for _, business := range businesses {
		var location *models.Location
		if business.AddressLocation != nil && business.AddressLocation.Valid {
			lat, lng := business.AddressLocation.P.Y, business.AddressLocation.P.X
			if anon {
				lat = math.Round(lat*100) / 100
				lng = math.Round(lng*100) / 100
			}
			location = &models.Location{
				Latitude:  lat,
				Longitude: lng,
				Country:   business.Country,
				Province:  business.Province,
				District:  business.District,
			}
		}

		categories := categoriesByBusiness[business.ID]
		if categories == nil {
			categories = []string{}
		}

		result := &models.DiscoverBusiness{
			ID:          business.ID,
			UserID:      business.UserID,
			Name:        business.Name,
			Description: business.Description,
			Avatar:      business.Avatar,
			Cover:       business.Cover,
			Location:    location,
			Categories:  categories,
			TotalFollow: business.TotalFollow,
		}

		results = append(results, result)
	}

	return results
}
