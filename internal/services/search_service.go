package services

import (
	"context"

	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/internal/repositories"
	"github.com/hamsaya/backend/internal/utils"
	"go.uber.org/zap"
)

// SearchService handles search and discovery operations
type SearchService struct {
	searchRepo       repositories.SearchRepository
	postRepo         repositories.PostRepository
	userRepo         repositories.UserRepository
	businessRepo     repositories.BusinessRepository
	categoryRepo     repositories.CategoryRepository
	relationshipsRepo repositories.RelationshipsRepository
	logger           *zap.Logger
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
		searchRepo:       searchRepo,
		postRepo:         postRepo,
		userRepo:         userRepo,
		businessRepo:     businessRepo,
		categoryRepo:     categoryRepo,
		relationshipsRepo: relationshipsRepo,
		logger:           logger,
	}
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

// Discover performs location-based discovery for map view
func (s *SearchService) Discover(ctx context.Context, userID *string, req *models.DiscoverRequest) (*models.DiscoverResponse, error) {
	// Set default limit
	limit := req.Limit
	if limit == 0 {
		limit = 100
	}

	response := &models.DiscoverResponse{
		Posts:      []*models.DiscoverPost{},
		Businesses: []*models.DiscoverBusiness{},
	}

	// Get posts within radius
	posts, err := s.searchRepo.GetDiscoverPosts(ctx, req.Latitude, req.Longitude, req.RadiusKm, req.Type, limit)
	if err != nil {
		s.logger.Error("Failed to get discover posts", zap.Error(err))
	} else {
		response.Posts = s.enrichDiscoverPosts(ctx, posts)
	}

	// Get businesses within radius
	businesses, err := s.searchRepo.GetDiscoverBusinesses(ctx, req.Latitude, req.Longitude, req.RadiusKm, limit)
	if err != nil {
		s.logger.Error("Failed to get discover businesses", zap.Error(err))
	} else {
		response.Businesses = s.enrichDiscoverBusinesses(ctx, businesses)
	}

	response.Total = len(response.Posts) + len(response.Businesses)

	s.logger.Info("Discovery completed",
		zap.Float64("latitude", req.Latitude),
		zap.Float64("longitude", req.Longitude),
		zap.Float64("radius_km", req.RadiusKm),
		zap.Int("total_results", response.Total),
	)

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
				author = &models.AuthorInfo{
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
			Attachments:    []models.Photo{}, // Will be populated if needed
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

// enrichDiscoverPosts enriches discover post results
func (s *SearchService) enrichDiscoverPosts(ctx context.Context, posts []*models.Post) []*models.DiscoverPost {
	var results []*models.DiscoverPost

	for _, post := range posts {
		// Extract location
		var location *models.Location
		if post.AddressLocation != nil {
			location = &models.Location{
				Country:  post.Country,
				Province: post.Province,
				District: post.District,
			}
		}

		// Format dates
		var startDate, startTime *string
		if post.StartDate != nil {
			dateStr := post.StartDate.Format("2006-01-02")
			startDate = &dateStr
		}
		if post.StartTime != nil {
			timeStr := post.StartTime.Format("15:04")
			startTime = &timeStr
		}

		result := &models.DiscoverPost{
			ID:          post.ID,
			Type:        post.Type,
			Title:       post.Title,
			Description: post.Description,
			Thumbnail:   nil, // Will be empty for now
			Location:    location,
			Distance:    0, // Distance is calculated in repository
			Price:       post.Price,
			StartDate:   startDate,
			StartTime:   startTime,
			CreatedAt:   post.CreatedAt,
		}

		results = append(results, result)
	}

	return results
}

// enrichDiscoverBusinesses enriches discover business results
func (s *SearchService) enrichDiscoverBusinesses(ctx context.Context, businesses []*models.BusinessProfile) []*models.DiscoverBusiness {
	var results []*models.DiscoverBusiness

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

		result := &models.DiscoverBusiness{
			ID:          business.ID,
			Name:        business.Name,
			Description: business.Description,
			Avatar:      business.Avatar,
			Location:    location,
			Distance:    0, // Distance is calculated in repository
			Categories:  []string{}, // Categories will be empty for now
			TotalFollow: business.TotalFollow,
		}

		results = append(results, result)
	}

	return results
}
