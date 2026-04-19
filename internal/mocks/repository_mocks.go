package mocks

import (
	"context"
	"time"

	"github.com/hamsaya/backend/internal/models"
	"github.com/stretchr/testify/mock"
)

// MockUserRepository is a mock implementation of UserRepository
type MockUserRepository struct {
	mock.Mock
}

func (m *MockUserRepository) Create(ctx context.Context, user *models.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockUserRepository) GetByID(ctx context.Context, id string) (*models.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockUserRepository) GetByIDIncludingDeleted(ctx context.Context, id string) (*models.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockUserRepository) GetByEmailIncludingDeleted(ctx context.Context, email string) (*models.User, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockUserRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockUserRepository) Update(ctx context.Context, user *models.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockUserRepository) UpdateLoginAttempts(ctx context.Context, userID string, attempts int, lockedUntil *time.Time) error {
	args := m.Called(ctx, userID, attempts, lockedUntil)
	return args.Error(0)
}

func (m *MockUserRepository) UpdateLastLogin(ctx context.Context, userID string) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *MockUserRepository) SoftDelete(ctx context.Context, userID string) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *MockUserRepository) Restore(ctx context.Context, userID string) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *MockUserRepository) CreateProfile(ctx context.Context, profile *models.Profile) error {
	args := m.Called(ctx, profile)
	return args.Error(0)
}

func (m *MockUserRepository) GetProfileByUserID(ctx context.Context, userID string) (*models.Profile, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Profile), args.Error(1)
}

func (m *MockUserRepository) GetProfileByUserIDIncludingDeleted(ctx context.Context, userID string) (*models.Profile, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Profile), args.Error(1)
}

func (m *MockUserRepository) UpdateProfile(ctx context.Context, profile *models.Profile) error {
	args := m.Called(ctx, profile)
	return args.Error(0)
}

func (m *MockUserRepository) CreateUserWithProfile(ctx context.Context, user *models.User, profile *models.Profile) error {
	args := m.Called(ctx, user, profile)
	return args.Error(0)
}

func (m *MockUserRepository) CreateSession(ctx context.Context, session *models.UserSession) error {
	args := m.Called(ctx, session)
	return args.Error(0)
}

func (m *MockUserRepository) GetSessionByID(ctx context.Context, sessionID string) (*models.UserSession, error) {
	args := m.Called(ctx, sessionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.UserSession), args.Error(1)
}

func (m *MockUserRepository) GetSessionByRefreshToken(ctx context.Context, refreshToken string) (*models.UserSession, error) {
	args := m.Called(ctx, refreshToken)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.UserSession), args.Error(1)
}

func (m *MockUserRepository) GetSessionByRefreshTokenHash(ctx context.Context, refreshTokenHash string) (*models.UserSession, error) {
	args := m.Called(ctx, refreshTokenHash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.UserSession), args.Error(1)
}

func (m *MockUserRepository) RevokeSession(ctx context.Context, sessionID string) error {
	args := m.Called(ctx, sessionID)
	return args.Error(0)
}

func (m *MockUserRepository) RevokeAllUserSessions(ctx context.Context, userID string) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *MockUserRepository) RevokeAllUserSessionsExcept(ctx context.Context, userID string, exceptSessionID string) error {
	args := m.Called(ctx, userID, exceptSessionID)
	return args.Error(0)
}

func (m *MockUserRepository) GetActiveSessions(ctx context.Context, userID string) ([]*models.UserSession, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.UserSession), args.Error(1)
}

// MockPostRepository is a mock implementation of PostRepository
type MockPostRepository struct {
	mock.Mock
}

func (m *MockPostRepository) GetByID(ctx context.Context, postID string) (*models.Post, error) {
	args := m.Called(ctx, postID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Post), args.Error(1)
}

func (m *MockPostRepository) Create(ctx context.Context, post *models.Post) error {
	args := m.Called(ctx, post)
	return args.Error(0)
}

func (m *MockPostRepository) Update(ctx context.Context, post *models.Post) error {
	args := m.Called(ctx, post)
	return args.Error(0)
}

func (m *MockPostRepository) Delete(ctx context.Context, postID string) error {
	args := m.Called(ctx, postID)
	return args.Error(0)
}

// Stub implementations for full PostRepository interface compliance
func (m *MockPostRepository) CreateAttachment(ctx context.Context, attachment *models.Attachment) error {
	args := m.Called(ctx, attachment)
	return args.Error(0)
}

func (m *MockPostRepository) GetAttachmentsByPostID(ctx context.Context, postID string) ([]*models.Attachment, error) {
	args := m.Called(ctx, postID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Attachment), args.Error(1)
}

func (m *MockPostRepository) DeleteAttachment(ctx context.Context, attachmentID string) error {
	args := m.Called(ctx, attachmentID)
	return args.Error(0)
}

func (m *MockPostRepository) DeleteAttachmentForPost(ctx context.Context, postID, attachmentID string) error {
	args := m.Called(ctx, postID, attachmentID)
	return args.Error(0)
}

func (m *MockPostRepository) LikePost(ctx context.Context, userID, postID string) error {
	args := m.Called(ctx, userID, postID)
	return args.Error(0)
}

func (m *MockPostRepository) UnlikePost(ctx context.Context, userID, postID string) error {
	args := m.Called(ctx, userID, postID)
	return args.Error(0)
}

func (m *MockPostRepository) IsLikedByUser(ctx context.Context, userID, postID string) (bool, error) {
	args := m.Called(ctx, userID, postID)
	return args.Bool(0), args.Error(1)
}

func (m *MockPostRepository) GetPostLikes(ctx context.Context, postID string, limit, offset int) ([]*models.PostLike, error) {
	args := m.Called(ctx, postID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.PostLike), args.Error(1)
}

func (m *MockPostRepository) BookmarkPost(ctx context.Context, userID, postID string) error {
	args := m.Called(ctx, userID, postID)
	return args.Error(0)
}

func (m *MockPostRepository) UnbookmarkPost(ctx context.Context, userID, postID string) error {
	args := m.Called(ctx, userID, postID)
	return args.Error(0)
}

func (m *MockPostRepository) IsBookmarkedByUser(ctx context.Context, userID, postID string) (bool, error) {
	args := m.Called(ctx, userID, postID)
	return args.Bool(0), args.Error(1)
}

func (m *MockPostRepository) SharePost(ctx context.Context, share *models.PostShare) error {
	args := m.Called(ctx, share)
	return args.Error(0)
}

func (m *MockPostRepository) GetPostShares(ctx context.Context, postID string, limit, offset int) ([]*models.PostShare, error) {
	args := m.Called(ctx, postID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.PostShare), args.Error(1)
}

func (m *MockPostRepository) GetEngagementStatus(ctx context.Context, userID, postID string) (bool, bool, error) {
	args := m.Called(ctx, userID, postID)
	return args.Bool(0), args.Bool(1), args.Error(2)
}

func (m *MockPostRepository) GetFeed(ctx context.Context, filter *models.FeedFilter) ([]*models.Post, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Post), args.Error(1)
}

func (m *MockPostRepository) GetUserPosts(ctx context.Context, userID string, limit, offset int) ([]*models.Post, error) {
	args := m.Called(ctx, userID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Post), args.Error(1)
}

func (m *MockPostRepository) GetUserBookmarks(ctx context.Context, userID string, limit, offset int) ([]*models.Post, error) {
	args := m.Called(ctx, userID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Post), args.Error(1)
}

func (m *MockPostRepository) GetUserEventPosts(ctx context.Context, userID string, eventState models.EventInterestState, limit, offset int) ([]*models.Post, error) {
	args := m.Called(ctx, userID, eventState, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Post), args.Error(1)
}

func (m *MockPostRepository) GetBusinessPosts(ctx context.Context, businessID string, limit, offset int) ([]*models.Post, error) {
	args := m.Called(ctx, businessID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Post), args.Error(1)
}

func (m *MockPostRepository) CountFeed(ctx context.Context, filter *models.FeedFilter) (int64, error) {
	args := m.Called(ctx, filter)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockPostRepository) CountPostsByUser(ctx context.Context, userID string) (int, error) {
	args := m.Called(ctx, userID)
	return args.Int(0), args.Error(1)
}

func (m *MockPostRepository) GetPostsByIDs(ctx context.Context, ids []string) ([]*models.Post, error) {
	args := m.Called(ctx, ids)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Post), args.Error(1)
}

func (m *MockPostRepository) ListExpiredSellPostsNeedingNotification(ctx context.Context, asOf time.Time) ([]*models.Post, error) {
	args := m.Called(ctx, asOf)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Post), args.Error(1)
}

func (m *MockPostRepository) MarkSellPostsExpired(ctx context.Context, postIDs []string) error {
	args := m.Called(ctx, postIDs)
	return args.Error(0)
}

func (m *MockPostRepository) ReactivateSellPost(ctx context.Context, postID string) error {
	args := m.Called(ctx, postID)
	return args.Error(0)
}

// MockReportRepository is a mock implementation of ReportRepository
type MockReportRepository struct {
	mock.Mock
}

func (m *MockReportRepository) CreatePostReport(ctx context.Context, report *models.PostReport) error {
	args := m.Called(ctx, report)
	return args.Error(0)
}

func (m *MockReportRepository) GetPostReport(ctx context.Context, id string) (*models.PostReport, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.PostReport), args.Error(1)
}

func (m *MockReportRepository) ListPostReports(ctx context.Context, limit, offset int) ([]*models.PostReport, int, error) {
	args := m.Called(ctx, limit, offset)
	if args.Get(0) == nil {
		return nil, 0, args.Error(2)
	}
	return args.Get(0).([]*models.PostReport), args.Int(1), args.Error(2)
}

func (m *MockReportRepository) UpdatePostReportStatus(ctx context.Context, id string, status models.ReportStatus) error {
	args := m.Called(ctx, id, status)
	return args.Error(0)
}

func (m *MockReportRepository) CreateCommentReport(ctx context.Context, report *models.CommentReport) error {
	args := m.Called(ctx, report)
	return args.Error(0)
}

func (m *MockReportRepository) GetCommentReport(ctx context.Context, id string) (*models.CommentReport, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.CommentReport), args.Error(1)
}

func (m *MockReportRepository) ListCommentReports(ctx context.Context, limit, offset int) ([]*models.CommentReport, int, error) {
	args := m.Called(ctx, limit, offset)
	if args.Get(0) == nil {
		return nil, 0, args.Error(2)
	}
	return args.Get(0).([]*models.CommentReport), args.Int(1), args.Error(2)
}

func (m *MockReportRepository) UpdateCommentReportStatus(ctx context.Context, id string, status models.ReportStatus) error {
	args := m.Called(ctx, id, status)
	return args.Error(0)
}

func (m *MockReportRepository) CreateUserReport(ctx context.Context, report *models.UserReport) error {
	args := m.Called(ctx, report)
	return args.Error(0)
}

func (m *MockReportRepository) GetUserReport(ctx context.Context, id string) (*models.UserReport, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.UserReport), args.Error(1)
}

func (m *MockReportRepository) ListUserReports(ctx context.Context, limit, offset int) ([]*models.UserReport, int, error) {
	args := m.Called(ctx, limit, offset)
	if args.Get(0) == nil {
		return nil, 0, args.Error(2)
	}
	return args.Get(0).([]*models.UserReport), args.Int(1), args.Error(2)
}

func (m *MockReportRepository) UpdateUserReportResolved(ctx context.Context, id string, resolved bool) error {
	args := m.Called(ctx, id, resolved)
	return args.Error(0)
}

func (m *MockReportRepository) CreateBusinessReport(ctx context.Context, report *models.BusinessReport) error {
	args := m.Called(ctx, report)
	return args.Error(0)
}

func (m *MockReportRepository) GetBusinessReport(ctx context.Context, id string) (*models.BusinessReport, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.BusinessReport), args.Error(1)
}

func (m *MockReportRepository) ListBusinessReports(ctx context.Context, limit, offset int) ([]*models.BusinessReport, int, error) {
	args := m.Called(ctx, limit, offset)
	if args.Get(0) == nil {
		return nil, 0, args.Error(2)
	}
	return args.Get(0).([]*models.BusinessReport), args.Int(1), args.Error(2)
}

func (m *MockReportRepository) UpdateBusinessReportStatus(ctx context.Context, id string, status models.ReportStatus) error {
	args := m.Called(ctx, id, status)
	return args.Error(0)
}

// MockRelationshipsRepository is a mock implementation of RelationshipsRepository
type MockRelationshipsRepository struct {
	mock.Mock
}

func (m *MockRelationshipsRepository) FollowUser(ctx context.Context, followerID, followingID string) error {
	args := m.Called(ctx, followerID, followingID)
	return args.Error(0)
}

func (m *MockRelationshipsRepository) UnfollowUser(ctx context.Context, followerID, followingID string) error {
	args := m.Called(ctx, followerID, followingID)
	return args.Error(0)
}

func (m *MockRelationshipsRepository) IsFollowing(ctx context.Context, followerID, followingID string) (bool, error) {
	args := m.Called(ctx, followerID, followingID)
	return args.Bool(0), args.Error(1)
}

func (m *MockRelationshipsRepository) GetFollowers(ctx context.Context, userID string, limit, offset int) ([]*models.UserFollow, error) {
	args := m.Called(ctx, userID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.UserFollow), args.Error(1)
}

func (m *MockRelationshipsRepository) GetFollowing(ctx context.Context, userID string, limit, offset int) ([]*models.UserFollow, error) {
	args := m.Called(ctx, userID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.UserFollow), args.Error(1)
}

func (m *MockRelationshipsRepository) GetFollowersCount(ctx context.Context, userID string) (int, error) {
	args := m.Called(ctx, userID)
	return args.Int(0), args.Error(1)
}

func (m *MockRelationshipsRepository) GetFollowingCount(ctx context.Context, userID string) (int, error) {
	args := m.Called(ctx, userID)
	return args.Int(0), args.Error(1)
}

func (m *MockRelationshipsRepository) BlockUser(ctx context.Context, blockerID, blockedID string) error {
	args := m.Called(ctx, blockerID, blockedID)
	return args.Error(0)
}

func (m *MockRelationshipsRepository) UnblockUser(ctx context.Context, blockerID, blockedID string) error {
	args := m.Called(ctx, blockerID, blockedID)
	return args.Error(0)
}

func (m *MockRelationshipsRepository) IsBlocked(ctx context.Context, blockerID, blockedID string) (bool, error) {
	args := m.Called(ctx, blockerID, blockedID)
	return args.Bool(0), args.Error(1)
}

func (m *MockRelationshipsRepository) GetBlockedUsers(ctx context.Context, blockerID string, limit, offset int) ([]*models.UserBlock, error) {
	args := m.Called(ctx, blockerID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.UserBlock), args.Error(1)
}

func (m *MockRelationshipsRepository) GetRelationshipStatus(ctx context.Context, viewerID, targetUserID string) (*models.RelationshipStatus, error) {
	args := m.Called(ctx, viewerID, targetUserID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.RelationshipStatus), args.Error(1)
}

// MockCommentRepository is a mock implementation of CommentRepository
type MockCommentRepository struct {
	mock.Mock
}

func (m *MockCommentRepository) Create(ctx context.Context, comment *models.PostComment) error {
	args := m.Called(ctx, comment)
	return args.Error(0)
}

func (m *MockCommentRepository) GetByID(ctx context.Context, commentID string) (*models.PostComment, error) {
	args := m.Called(ctx, commentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.PostComment), args.Error(1)
}

func (m *MockCommentRepository) Update(ctx context.Context, comment *models.PostComment) error {
	args := m.Called(ctx, comment)
	return args.Error(0)
}

func (m *MockCommentRepository) Delete(ctx context.Context, commentID string) error {
	args := m.Called(ctx, commentID)
	return args.Error(0)
}

func (m *MockCommentRepository) GetByPostID(ctx context.Context, postID string, limit, offset int) ([]*models.PostComment, error) {
	args := m.Called(ctx, postID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.PostComment), args.Error(1)
}

func (m *MockCommentRepository) GetReplies(ctx context.Context, parentCommentID string, limit, offset int) ([]*models.PostComment, error) {
	args := m.Called(ctx, parentCommentID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.PostComment), args.Error(1)
}

func (m *MockCommentRepository) CountByPostID(ctx context.Context, postID string) (int, error) {
	args := m.Called(ctx, postID)
	return args.Int(0), args.Error(1)
}

func (m *MockCommentRepository) CreateAttachment(ctx context.Context, attachment *models.CommentAttachment) error {
	args := m.Called(ctx, attachment)
	return args.Error(0)
}

func (m *MockCommentRepository) GetAttachmentsByCommentID(ctx context.Context, commentID string) ([]*models.CommentAttachment, error) {
	args := m.Called(ctx, commentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.CommentAttachment), args.Error(1)
}

func (m *MockCommentRepository) DeleteAttachment(ctx context.Context, attachmentID string) error {
	args := m.Called(ctx, attachmentID)
	return args.Error(0)
}

func (m *MockCommentRepository) LikeComment(ctx context.Context, userID, commentID string) error {
	args := m.Called(ctx, userID, commentID)
	return args.Error(0)
}

func (m *MockCommentRepository) UnlikeComment(ctx context.Context, userID, commentID string) error {
	args := m.Called(ctx, userID, commentID)
	return args.Error(0)
}

func (m *MockCommentRepository) IsLikedByUser(ctx context.Context, userID, commentID string) (bool, error) {
	args := m.Called(ctx, userID, commentID)
	return args.Bool(0), args.Error(1)
}

func (m *MockCommentRepository) GetCommentLikes(ctx context.Context, commentID string, limit, offset int) ([]*models.CommentLike, error) {
	args := m.Called(ctx, commentID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.CommentLike), args.Error(1)
}

// MockBusinessRepository is a mock implementation of BusinessRepository
type MockBusinessRepository struct {
	mock.Mock
}

func (m *MockBusinessRepository) Create(ctx context.Context, business *models.BusinessProfile) error {
	args := m.Called(ctx, business)
	return args.Error(0)
}

func (m *MockBusinessRepository) GetByID(ctx context.Context, businessID string) (*models.BusinessProfile, error) {
	args := m.Called(ctx, businessID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.BusinessProfile), args.Error(1)
}

func (m *MockBusinessRepository) GetByUserID(ctx context.Context, userID string, limit, offset int) ([]*models.BusinessProfile, error) {
	args := m.Called(ctx, userID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.BusinessProfile), args.Error(1)
}

func (m *MockBusinessRepository) Update(ctx context.Context, business *models.BusinessProfile) error {
	args := m.Called(ctx, business)
	return args.Error(0)
}

func (m *MockBusinessRepository) Delete(ctx context.Context, businessID string) error {
	args := m.Called(ctx, businessID)
	return args.Error(0)
}

func (m *MockBusinessRepository) List(ctx context.Context, filter *models.BusinessListFilter) ([]*models.BusinessProfile, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.BusinessProfile), args.Error(1)
}

func (m *MockBusinessRepository) GetCategoriesByBusinessID(ctx context.Context, businessID string) ([]*models.BusinessCategory, error) {
	args := m.Called(ctx, businessID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.BusinessCategory), args.Error(1)
}

func (m *MockBusinessRepository) AddCategories(ctx context.Context, businessID string, categoryIDs []string) error {
	args := m.Called(ctx, businessID, categoryIDs)
	return args.Error(0)
}

func (m *MockBusinessRepository) RemoveCategories(ctx context.Context, businessID string) error {
	args := m.Called(ctx, businessID)
	return args.Error(0)
}

func (m *MockBusinessRepository) GetHoursByBusinessID(ctx context.Context, businessID string) ([]*models.BusinessHours, error) {
	args := m.Called(ctx, businessID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.BusinessHours), args.Error(1)
}

func (m *MockBusinessRepository) UpsertHours(ctx context.Context, hours *models.BusinessHours) error {
	args := m.Called(ctx, hours)
	return args.Error(0)
}

func (m *MockBusinessRepository) DeleteHoursByBusinessID(ctx context.Context, businessID string) error {
	args := m.Called(ctx, businessID)
	return args.Error(0)
}

func (m *MockBusinessRepository) AddAttachment(ctx context.Context, attachment *models.BusinessAttachment) error {
	args := m.Called(ctx, attachment)
	return args.Error(0)
}

func (m *MockBusinessRepository) GetAttachmentsByBusinessID(ctx context.Context, businessID string) ([]*models.BusinessAttachment, error) {
	args := m.Called(ctx, businessID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.BusinessAttachment), args.Error(1)
}

func (m *MockBusinessRepository) DeleteAttachment(ctx context.Context, attachmentID string) error {
	args := m.Called(ctx, attachmentID)
	return args.Error(0)
}

func (m *MockBusinessRepository) Follow(ctx context.Context, businessID, userID string) error {
	args := m.Called(ctx, businessID, userID)
	return args.Error(0)
}

func (m *MockBusinessRepository) Unfollow(ctx context.Context, businessID, userID string) error {
	args := m.Called(ctx, businessID, userID)
	return args.Error(0)
}

func (m *MockBusinessRepository) IsFollowing(ctx context.Context, businessID, userID string) (bool, error) {
	args := m.Called(ctx, businessID, userID)
	return args.Bool(0), args.Error(1)
}

func (m *MockBusinessRepository) GetFollowers(ctx context.Context, businessID string, limit, offset int) ([]string, error) {
	args := m.Called(ctx, businessID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockBusinessRepository) GetAllCategories(ctx context.Context, search *string) ([]*models.BusinessCategory, error) {
	args := m.Called(ctx, search)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.BusinessCategory), args.Error(1)
}

func (m *MockBusinessRepository) GetOrCreateCategoryByName(ctx context.Context, name string) (string, error) {
	args := m.Called(ctx, name)
	return args.String(0), args.Error(1)
}

func (m *MockBusinessRepository) IncrementViews(ctx context.Context, businessID string) error {
	args := m.Called(ctx, businessID)
	return args.Error(0)
}

// MockNotificationRepository is a mock implementation of NotificationRepository
type MockNotificationRepository struct {
	mock.Mock
}

func (m *MockNotificationRepository) Create(ctx context.Context, notification *models.Notification) error {
	args := m.Called(ctx, notification)
	return args.Error(0)
}

func (m *MockNotificationRepository) GetByID(ctx context.Context, notificationID string) (*models.Notification, error) {
	args := m.Called(ctx, notificationID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Notification), args.Error(1)
}

func (m *MockNotificationRepository) List(ctx context.Context, filter *models.GetNotificationsFilter) ([]*models.Notification, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Notification), args.Error(1)
}

func (m *MockNotificationRepository) MarkAsRead(ctx context.Context, notificationID string) error {
	args := m.Called(ctx, notificationID)
	return args.Error(0)
}

func (m *MockNotificationRepository) MarkAllAsRead(ctx context.Context, userID string) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *MockNotificationRepository) Delete(ctx context.Context, notificationID string) error {
	args := m.Called(ctx, notificationID)
	return args.Error(0)
}

func (m *MockNotificationRepository) GetUnreadCount(ctx context.Context, userID string, businessID *string) (int, error) {
	args := m.Called(ctx, userID, businessID)
	return args.Int(0), args.Error(1)
}

// MockNotificationSettingsRepository is a mock implementation of NotificationSettingsRepository
type MockNotificationSettingsRepository struct {
	mock.Mock
}

func (m *MockNotificationSettingsRepository) GetByProfileID(ctx context.Context, profileID string) ([]*models.NotificationSetting, error) {
	args := m.Called(ctx, profileID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.NotificationSetting), args.Error(1)
}

func (m *MockNotificationSettingsRepository) GetByProfileAndCategory(ctx context.Context, profileID string, category models.NotificationCategory) (*models.NotificationSetting, error) {
	args := m.Called(ctx, profileID, category)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.NotificationSetting), args.Error(1)
}

func (m *MockNotificationSettingsRepository) Upsert(ctx context.Context, setting *models.NotificationSetting) error {
	args := m.Called(ctx, setting)
	return args.Error(0)
}

func (m *MockNotificationSettingsRepository) UpdateCategory(ctx context.Context, profileID string, category models.NotificationCategory, pushPref bool) error {
	args := m.Called(ctx, profileID, category, pushPref)
	return args.Error(0)
}

func (m *MockNotificationSettingsRepository) UpsertCategory(ctx context.Context, profileID string, category models.NotificationCategory, pushPref bool) error {
	args := m.Called(ctx, profileID, category, pushPref)
	return args.Error(0)
}

func (m *MockNotificationSettingsRepository) InitializeDefaults(ctx context.Context, profileID string) error {
	args := m.Called(ctx, profileID)
	return args.Error(0)
}

// MockAdminRepository is a mock implementation of AdminRepository
type MockAdminRepository struct {
	mock.Mock
}

func (m *MockAdminRepository) GetDashboardStats(ctx context.Context) (*models.DashboardStats, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.DashboardStats), args.Error(1)
}

func (m *MockAdminRepository) GetUserAnalytics(ctx context.Context, period string) (*models.UserAnalytics, error) {
	args := m.Called(ctx, period)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.UserAnalytics), args.Error(1)
}

func (m *MockAdminRepository) GetPostAnalytics(ctx context.Context, period string) (*models.PostAnalytics, error) {
	args := m.Called(ctx, period)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.PostAnalytics), args.Error(1)
}

func (m *MockAdminRepository) GetEngagementAnalytics(ctx context.Context, period string) (*models.EngagementAnalytics, error) {
	args := m.Called(ctx, period)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.EngagementAnalytics), args.Error(1)
}

func (m *MockAdminRepository) ListUsers(ctx context.Context, filter *models.AdminUserFilter) ([]*models.AdminUserResponse, int64, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Get(1).(int64), args.Error(2)
	}
	return args.Get(0).([]*models.AdminUserResponse), args.Get(1).(int64), args.Error(2)
}

func (m *MockAdminRepository) GetUserByID(ctx context.Context, userID string) (*models.AdminUserResponse, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.AdminUserResponse), args.Error(1)
}

func (m *MockAdminRepository) GetUserBio(ctx context.Context, userID string) (*string, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*string), args.Error(1)
}

func (m *MockAdminRepository) GetUserPosts(ctx context.Context, userID string, limit int) ([]*models.AdminPostResponse, error) {
	args := m.Called(ctx, userID, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.AdminPostResponse), args.Error(1)
}

func (m *MockAdminRepository) GetUserBusinesses(ctx context.Context, userID string) ([]*models.AdminBusinessResponse, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.AdminBusinessResponse), args.Error(1)
}

func (m *MockAdminRepository) SuspendUser(ctx context.Context, userID string, until time.Time) error {
	args := m.Called(ctx, userID, until)
	return args.Error(0)
}

func (m *MockAdminRepository) UnsuspendUser(ctx context.Context, userID string) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *MockAdminRepository) UpdateUserRole(ctx context.Context, userID string, role models.UserRole) error {
	args := m.Called(ctx, userID, role)
	return args.Error(0)
}

func (m *MockAdminRepository) DeleteUser(ctx context.Context, userID string) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *MockAdminRepository) ListPosts(ctx context.Context, filter *models.AdminPostFilter) ([]*models.AdminPostResponse, int64, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Get(1).(int64), args.Error(2)
	}
	return args.Get(0).([]*models.AdminPostResponse), args.Get(1).(int64), args.Error(2)
}

func (m *MockAdminRepository) GetPostByID(ctx context.Context, postID string) (*models.AdminPostDetailResponse, error) {
	args := m.Called(ctx, postID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.AdminPostDetailResponse), args.Error(1)
}

func (m *MockAdminRepository) GetPostComments(ctx context.Context, postID string) ([]models.AdminPostCommentResponse, error) {
	args := m.Called(ctx, postID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.AdminPostCommentResponse), args.Error(1)
}

func (m *MockAdminRepository) UpdatePostStatus(ctx context.Context, postID, status string) error {
	args := m.Called(ctx, postID, status)
	return args.Error(0)
}

func (m *MockAdminRepository) DeletePost(ctx context.Context, postID string) error {
	args := m.Called(ctx, postID)
	return args.Error(0)
}

func (m *MockAdminRepository) ListComments(ctx context.Context, filter *models.AdminCommentFilter) ([]*models.AdminCommentResponse, int64, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Get(1).(int64), args.Error(2)
	}
	return args.Get(0).([]*models.AdminCommentResponse), args.Get(1).(int64), args.Error(2)
}

func (m *MockAdminRepository) GetCommentByID(ctx context.Context, commentID string) (*models.AdminCommentDetailResponse, error) {
	args := m.Called(ctx, commentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.AdminCommentDetailResponse), args.Error(1)
}

func (m *MockAdminRepository) DeleteComment(ctx context.Context, commentID string) error {
	args := m.Called(ctx, commentID)
	return args.Error(0)
}

func (m *MockAdminRepository) RestoreComment(ctx context.Context, commentID string) error {
	args := m.Called(ctx, commentID)
	return args.Error(0)
}

func (m *MockAdminRepository) ResolveCommentReportsByCommentID(ctx context.Context, commentID string) error {
	args := m.Called(ctx, commentID)
	return args.Error(0)
}

func (m *MockAdminRepository) ListBusinesses(ctx context.Context, filter *models.AdminBusinessFilter) ([]*models.AdminBusinessResponse, int64, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Get(1).(int64), args.Error(2)
	}
	return args.Get(0).([]*models.AdminBusinessResponse), args.Get(1).(int64), args.Error(2)
}

func (m *MockAdminRepository) GetBusinessByID(ctx context.Context, businessID string) (*models.AdminBusinessDetailResponse, error) {
	args := m.Called(ctx, businessID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.AdminBusinessDetailResponse), args.Error(1)
}

func (m *MockAdminRepository) GetBusinessPosts(ctx context.Context, businessID string, limit int) ([]*models.AdminPostResponse, error) {
	args := m.Called(ctx, businessID, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.AdminPostResponse), args.Error(1)
}

func (m *MockAdminRepository) GetBusinessHours(ctx context.Context, businessID string) ([]models.AdminBusinessHour, error) {
	args := m.Called(ctx, businessID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.AdminBusinessHour), args.Error(1)
}

func (m *MockAdminRepository) GetBusinessCategories(ctx context.Context, businessID string) ([]string, error) {
	args := m.Called(ctx, businessID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockAdminRepository) GetBusinessGallery(ctx context.Context, businessID string) ([]models.AttachmentResponse, error) {
	args := m.Called(ctx, businessID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.AttachmentResponse), args.Error(1)
}

func (m *MockAdminRepository) UpdateBusinessStatus(ctx context.Context, businessID, status string) error {
	args := m.Called(ctx, businessID, status)
	return args.Error(0)
}

func (m *MockAdminRepository) DeleteBusiness(ctx context.Context, businessID string) error {
	args := m.Called(ctx, businessID)
	return args.Error(0)
}

func (m *MockAdminRepository) ListPostReports(ctx context.Context, filter *models.AdminReportFilter) ([]*models.AdminPostReportResponse, int64, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Get(1).(int64), args.Error(2)
	}
	return args.Get(0).([]*models.AdminPostReportResponse), args.Get(1).(int64), args.Error(2)
}

func (m *MockAdminRepository) GetPostReportByID(ctx context.Context, reportID string) (*models.AdminPostReportResponse, error) {
	args := m.Called(ctx, reportID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.AdminPostReportResponse), args.Error(1)
}

func (m *MockAdminRepository) ListCommentReports(ctx context.Context, filter *models.AdminReportFilter) ([]*models.AdminCommentReportResponse, int64, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Get(1).(int64), args.Error(2)
	}
	return args.Get(0).([]*models.AdminCommentReportResponse), args.Get(1).(int64), args.Error(2)
}

func (m *MockAdminRepository) GetCommentReportByID(ctx context.Context, reportID string) (*models.AdminCommentReportResponse, error) {
	args := m.Called(ctx, reportID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.AdminCommentReportResponse), args.Error(1)
}

func (m *MockAdminRepository) ListUserReports(ctx context.Context, filter *models.AdminReportFilter) ([]*models.AdminUserReportResponse, int64, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Get(1).(int64), args.Error(2)
	}
	return args.Get(0).([]*models.AdminUserReportResponse), args.Get(1).(int64), args.Error(2)
}

func (m *MockAdminRepository) GetUserReportByID(ctx context.Context, reportID string) (*models.AdminUserReportResponse, error) {
	args := m.Called(ctx, reportID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.AdminUserReportResponse), args.Error(1)
}

func (m *MockAdminRepository) ListBusinessReports(ctx context.Context, filter *models.AdminReportFilter) ([]*models.AdminBusinessReportResponse, int64, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Get(1).(int64), args.Error(2)
	}
	return args.Get(0).([]*models.AdminBusinessReportResponse), args.Get(1).(int64), args.Error(2)
}

func (m *MockAdminRepository) GetBusinessReportByID(ctx context.Context, reportID string) (*models.AdminBusinessReportResponse, error) {
	args := m.Called(ctx, reportID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.AdminBusinessReportResponse), args.Error(1)
}

func (m *MockAdminRepository) UpdatePostReportStatus(ctx context.Context, reportID, status string) error {
	args := m.Called(ctx, reportID, status)
	return args.Error(0)
}

func (m *MockAdminRepository) UpdateCommentReportStatus(ctx context.Context, reportID, status string) error {
	args := m.Called(ctx, reportID, status)
	return args.Error(0)
}

func (m *MockAdminRepository) UpdateUserReportResolved(ctx context.Context, reportID string, resolved bool) error {
	args := m.Called(ctx, reportID, resolved)
	return args.Error(0)
}

func (m *MockAdminRepository) UpdateBusinessReportStatus(ctx context.Context, reportID, status string) error {
	args := m.Called(ctx, reportID, status)
	return args.Error(0)
}

func (m *MockAdminRepository) GetAllUserIDs(ctx context.Context) ([]string, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockAdminRepository) GetUserIDsByProvince(ctx context.Context, province string) ([]string, error) {
	args := m.Called(ctx, province)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockAdminRepository) ListFeedback(ctx context.Context, filter *models.AdminFeedbackFilter) ([]*models.AdminFeedbackResponse, int64, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Get(1).(int64), args.Error(2)
	}
	return args.Get(0).([]*models.AdminFeedbackResponse), args.Get(1).(int64), args.Error(2)
}

func (m *MockAdminRepository) ResolveFeedback(ctx context.Context, feedbackID, adminID, status string, notes *string) error {
	args := m.Called(ctx, feedbackID, adminID, status, notes)
	return args.Error(0)
}

func (m *MockAdminRepository) GetBusinessAnalytics(ctx context.Context, period string) (*models.BusinessAnalytics, error) {
	args := m.Called(ctx, period)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.BusinessAnalytics), args.Error(1)
}

func (m *MockAdminRepository) CreateAuditLog(ctx context.Context, req *models.CreateAuditLogRequest) error {
	args := m.Called(ctx, req)
	return args.Error(0)
}

func (m *MockAdminRepository) ListAuditLogs(ctx context.Context, filter *models.AuditLogFilter) ([]*models.AuditLog, int64, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Get(1).(int64), args.Error(2)
	}
	return args.Get(0).([]*models.AuditLog), args.Get(1).(int64), args.Error(2)
}

func (m *MockAdminRepository) CreateAdminInvite(ctx context.Context, email, token, role, invitedBy string, expiresAt time.Time) error {
	args := m.Called(ctx, email, token, role, invitedBy, expiresAt)
	return args.Error(0)
}

func (m *MockAdminRepository) ListAdminInvites(ctx context.Context) ([]*models.AdminInvite, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.AdminInvite), args.Error(1)
}

func (m *MockAdminRepository) RevokeAdminInvite(ctx context.Context, inviteID string) error {
	args := m.Called(ctx, inviteID)
	return args.Error(0)
}

func (m *MockAdminRepository) GetAdminInviteByToken(ctx context.Context, token string) (*models.AdminInvite, error) {
	args := m.Called(ctx, token)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.AdminInvite), args.Error(1)
}

func (m *MockAdminRepository) UseAdminInvite(ctx context.Context, token string) error {
	args := m.Called(ctx, token)
	return args.Error(0)
}

func (m *MockAdminRepository) ListAdmins(ctx context.Context) ([]*models.AdminActiveUser, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.AdminActiveUser), args.Error(1)
}

func (m *MockAdminRepository) CreateIPBan(ctx context.Context, ipAddress, bannedBy string, reason *string, expiresAt *time.Time) error {
	args := m.Called(ctx, ipAddress, bannedBy, reason, expiresAt)
	return args.Error(0)
}

func (m *MockAdminRepository) ListIPBans(ctx context.Context, page, limit int) ([]*models.IPBan, int64, error) {
	args := m.Called(ctx, page, limit)
	if args.Get(0) == nil {
		return nil, args.Get(1).(int64), args.Error(2)
	}
	return args.Get(0).([]*models.IPBan), args.Get(1).(int64), args.Error(2)
}

func (m *MockAdminRepository) DeleteIPBan(ctx context.Context, banID string) error {
	args := m.Called(ctx, banID)
	return args.Error(0)
}

func (m *MockAdminRepository) IsIPBanned(ctx context.Context, ipAddress string) (bool, error) {
	args := m.Called(ctx, ipAddress)
	return args.Bool(0), args.Error(1)
}

func (m *MockAdminRepository) CreateDeviceBan(ctx context.Context, deviceID, bannedBy string, reason *string, expiresAt *time.Time) error {
	args := m.Called(ctx, deviceID, bannedBy, reason, expiresAt)
	return args.Error(0)
}

func (m *MockAdminRepository) ListDeviceBans(ctx context.Context, page, limit int) ([]*models.DeviceBan, int64, error) {
	args := m.Called(ctx, page, limit)
	if args.Get(0) == nil {
		return nil, args.Get(1).(int64), args.Error(2)
	}
	return args.Get(0).([]*models.DeviceBan), args.Get(1).(int64), args.Error(2)
}

func (m *MockAdminRepository) DeleteDeviceBan(ctx context.Context, banID string) error {
	args := m.Called(ctx, banID)
	return args.Error(0)
}

func (m *MockAdminRepository) IsDeviceBanned(ctx context.Context, deviceID string) (bool, error) {
	args := m.Called(ctx, deviceID)
	return args.Bool(0), args.Error(1)
}

// MockCategoryRepository is a mock implementation of CategoryRepository
type MockCategoryRepository struct {
	mock.Mock
}

func (m *MockCategoryRepository) Create(ctx context.Context, category *models.SellCategory) error {
	args := m.Called(ctx, category)
	return args.Error(0)
}

func (m *MockCategoryRepository) GetByID(ctx context.Context, categoryID string) (*models.SellCategory, error) {
	args := m.Called(ctx, categoryID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.SellCategory), args.Error(1)
}

func (m *MockCategoryRepository) GetAll(ctx context.Context) ([]*models.SellCategory, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.SellCategory), args.Error(1)
}

func (m *MockCategoryRepository) Update(ctx context.Context, category *models.SellCategory) error {
	args := m.Called(ctx, category)
	return args.Error(0)
}

func (m *MockCategoryRepository) Delete(ctx context.Context, categoryID string) error {
	args := m.Called(ctx, categoryID)
	return args.Error(0)
}

func (m *MockCategoryRepository) List(ctx context.Context, filter *models.CategoryListFilter) ([]*models.SellCategory, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.SellCategory), args.Error(1)
}

func (m *MockCategoryRepository) GetByIDs(ctx context.Context, categoryIDs []string) ([]*models.SellCategory, error) {
	args := m.Called(ctx, categoryIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.SellCategory), args.Error(1)
}

func (m *MockCategoryRepository) GetActiveCategories(ctx context.Context) ([]*models.SellCategory, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.SellCategory), args.Error(1)
}

// MockFeedbackRepository is a mock implementation of FeedbackRepository
type MockFeedbackRepository struct {
	mock.Mock
}

func (m *MockFeedbackRepository) Create(ctx context.Context, feedback *models.Feedback) error {
	args := m.Called(ctx, feedback)
	return args.Error(0)
}

func (m *MockFeedbackRepository) GetUserFeedbackStatus(ctx context.Context, userID string) (bool, *time.Time, error) {
	args := m.Called(ctx, userID)
	if args.Get(1) == nil {
		return args.Bool(0), nil, args.Error(2)
	}
	return args.Bool(0), args.Get(1).(*time.Time), args.Error(2)
}

func (m *MockFeedbackRepository) GetByUserID(ctx context.Context, userID string, limit, offset int) ([]*models.Feedback, error) {
	args := m.Called(ctx, userID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Feedback), args.Error(1)
}

// MockEventRepository is a mock implementation of EventRepository
type MockEventRepository struct {
	mock.Mock
}

func (m *MockEventRepository) SetInterest(ctx context.Context, interest *models.EventInterest) error {
	args := m.Called(ctx, interest)
	return args.Error(0)
}

func (m *MockEventRepository) GetUserInterest(ctx context.Context, userID, postID string) (*models.EventInterest, error) {
	args := m.Called(ctx, userID, postID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.EventInterest), args.Error(1)
}

func (m *MockEventRepository) DeleteInterest(ctx context.Context, userID, postID string) error {
	args := m.Called(ctx, userID, postID)
	return args.Error(0)
}

func (m *MockEventRepository) GetInterestedUsers(ctx context.Context, postID string, state models.EventInterestState, limit, offset int) ([]*models.EventInterest, error) {
	args := m.Called(ctx, postID, state, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.EventInterest), args.Error(1)
}

func (m *MockEventRepository) CountByState(ctx context.Context, postID string, state models.EventInterestState) (int, error) {
	args := m.Called(ctx, postID, state)
	return args.Int(0), args.Error(1)
}

// MockPollRepository is a mock implementation of PollRepository
type MockPollRepository struct {
	mock.Mock
}

func (m *MockPollRepository) Create(ctx context.Context, poll *models.Poll) error {
	args := m.Called(ctx, poll)
	return args.Error(0)
}

func (m *MockPollRepository) GetByID(ctx context.Context, pollID string) (*models.Poll, error) {
	args := m.Called(ctx, pollID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Poll), args.Error(1)
}

func (m *MockPollRepository) GetByPostID(ctx context.Context, postID string) (*models.Poll, error) {
	args := m.Called(ctx, postID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Poll), args.Error(1)
}

func (m *MockPollRepository) Delete(ctx context.Context, pollID string) error {
	args := m.Called(ctx, pollID)
	return args.Error(0)
}

func (m *MockPollRepository) CreateOption(ctx context.Context, option *models.PollOption) error {
	args := m.Called(ctx, option)
	return args.Error(0)
}

func (m *MockPollRepository) GetOptionsByPollID(ctx context.Context, pollID string) ([]*models.PollOption, error) {
	args := m.Called(ctx, pollID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.PollOption), args.Error(1)
}

func (m *MockPollRepository) GetOptionByID(ctx context.Context, optionID string) (*models.PollOption, error) {
	args := m.Called(ctx, optionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.PollOption), args.Error(1)
}

func (m *MockPollRepository) UpdateOptionVoteCount(ctx context.Context, optionID string, increment int) error {
	args := m.Called(ctx, optionID, increment)
	return args.Error(0)
}

func (m *MockPollRepository) DeleteOptionsByPollID(ctx context.Context, pollID string) error {
	args := m.Called(ctx, pollID)
	return args.Error(0)
}

func (m *MockPollRepository) VotePoll(ctx context.Context, vote *models.UserPoll) error {
	args := m.Called(ctx, vote)
	return args.Error(0)
}

func (m *MockPollRepository) ChangeVote(ctx context.Context, userID, pollID, newOptionID string) error {
	args := m.Called(ctx, userID, pollID, newOptionID)
	return args.Error(0)
}

func (m *MockPollRepository) DeleteVote(ctx context.Context, userID, pollID string) error {
	args := m.Called(ctx, userID, pollID)
	return args.Error(0)
}

func (m *MockPollRepository) GetUserVote(ctx context.Context, userID, pollID string) (*models.UserPoll, error) {
	args := m.Called(ctx, userID, pollID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.UserPoll), args.Error(1)
}

func (m *MockPollRepository) HasUserVoted(ctx context.Context, userID, pollID string) (bool, error) {
	args := m.Called(ctx, userID, pollID)
	return args.Bool(0), args.Error(1)
}

// MockConversationRepository is a mock implementation of ConversationRepository
type MockConversationRepository struct {
	mock.Mock
}

func (m *MockConversationRepository) GetOrCreate(ctx context.Context, userID1, userID2 string) (*models.Conversation, error) {
	args := m.Called(ctx, userID1, userID2)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Conversation), args.Error(1)
}

func (m *MockConversationRepository) GetByID(ctx context.Context, conversationID string) (*models.Conversation, error) {
	args := m.Called(ctx, conversationID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Conversation), args.Error(1)
}

func (m *MockConversationRepository) GetByParticipants(ctx context.Context, userID1, userID2 string) (*models.Conversation, error) {
	args := m.Called(ctx, userID1, userID2)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Conversation), args.Error(1)
}

func (m *MockConversationRepository) List(ctx context.Context, filter *models.GetConversationsFilter) ([]*models.Conversation, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Conversation), args.Error(1)
}

func (m *MockConversationRepository) UpdateLastMessageAt(ctx context.Context, conversationID string) error {
	args := m.Called(ctx, conversationID)
	return args.Error(0)
}

func (m *MockConversationRepository) Delete(ctx context.Context, conversationID string) error {
	args := m.Called(ctx, conversationID)
	return args.Error(0)
}

func (m *MockConversationRepository) IsParticipant(ctx context.Context, conversationID, userID string) (bool, error) {
	args := m.Called(ctx, conversationID, userID)
	return args.Bool(0), args.Error(1)
}

func (m *MockConversationRepository) GetOtherParticipantID(ctx context.Context, conversationID, userID string) (string, error) {
	args := m.Called(ctx, conversationID, userID)
	return args.String(0), args.Error(1)
}

// MockMessageRepository is a mock implementation of MessageRepository
type MockMessageRepository struct {
	mock.Mock
}

func (m *MockMessageRepository) Create(ctx context.Context, message *models.Message) error {
	args := m.Called(ctx, message)
	return args.Error(0)
}

func (m *MockMessageRepository) GetByID(ctx context.Context, messageID string) (*models.Message, error) {
	args := m.Called(ctx, messageID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Message), args.Error(1)
}

func (m *MockMessageRepository) List(ctx context.Context, filter *models.GetMessagesFilter) ([]*models.Message, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Message), args.Error(1)
}

func (m *MockMessageRepository) Delete(ctx context.Context, messageID string) error {
	args := m.Called(ctx, messageID)
	return args.Error(0)
}

func (m *MockMessageRepository) MarkAsRead(ctx context.Context, messageID string) error {
	args := m.Called(ctx, messageID)
	return args.Error(0)
}

func (m *MockMessageRepository) MarkConversationAsRead(ctx context.Context, conversationID, userID string) error {
	args := m.Called(ctx, conversationID, userID)
	return args.Error(0)
}

func (m *MockMessageRepository) GetUnreadCount(ctx context.Context, conversationID, userID string) (int, error) {
	args := m.Called(ctx, conversationID, userID)
	return args.Int(0), args.Error(1)
}

func (m *MockMessageRepository) GetLastMessage(ctx context.Context, conversationID string) (*models.Message, error) {
	args := m.Called(ctx, conversationID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Message), args.Error(1)
}

// MockMFARepository is a mock implementation of MFARepository
type MockMFARepository struct {
	mock.Mock
}

func (m *MockMFARepository) CreateFactor(ctx context.Context, factor *models.MFAFactor) error {
	args := m.Called(ctx, factor)
	return args.Error(0)
}

func (m *MockMFARepository) GetFactorByID(ctx context.Context, factorID string) (*models.MFAFactor, error) {
	args := m.Called(ctx, factorID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.MFAFactor), args.Error(1)
}

func (m *MockMFARepository) GetFactorsByUserID(ctx context.Context, userID string) ([]*models.MFAFactor, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.MFAFactor), args.Error(1)
}

func (m *MockMFARepository) UpdateFactorStatus(ctx context.Context, factorID, status string) error {
	args := m.Called(ctx, factorID, status)
	return args.Error(0)
}

func (m *MockMFARepository) DeleteFactor(ctx context.Context, factorID string) error {
	args := m.Called(ctx, factorID)
	return args.Error(0)
}

func (m *MockMFARepository) CreateBackupCodes(ctx context.Context, codes []*models.BackupCode) error {
	args := m.Called(ctx, codes)
	return args.Error(0)
}

func (m *MockMFARepository) GetBackupCode(ctx context.Context, userID, code string) (*models.BackupCode, error) {
	args := m.Called(ctx, userID, code)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.BackupCode), args.Error(1)
}

func (m *MockMFARepository) MarkBackupCodeAsUsed(ctx context.Context, codeID string) error {
	args := m.Called(ctx, codeID)
	return args.Error(0)
}

func (m *MockMFARepository) GetUnusedBackupCodesCount(ctx context.Context, userID string) (int, error) {
	args := m.Called(ctx, userID)
	return args.Int(0), args.Error(1)
}

func (m *MockMFARepository) DeleteAllBackupCodes(ctx context.Context, userID string) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

// MockFanoutRepository is a mock implementation of FanoutRepository
type MockFanoutRepository struct {
	mock.Mock
}

func (m *MockFanoutRepository) InsertFeedEntries(ctx context.Context, postID string, followerIDs []string) error {
	args := m.Called(ctx, postID, followerIDs)
	return args.Error(0)
}

func (m *MockFanoutRepository) GetFollowerIDs(ctx context.Context, authorID string) ([]string, error) {
	args := m.Called(ctx, authorID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockFanoutRepository) CountFollowers(ctx context.Context, authorID string) (int, error) {
	args := m.Called(ctx, authorID)
	return args.Int(0), args.Error(1)
}

func (m *MockFanoutRepository) GetPersonalizedFeed(ctx context.Context, viewerID string, cursor *time.Time, limit int) ([]string, *time.Time, error) {
	args := m.Called(ctx, viewerID, cursor, limit)
	var postIDs []string
	if args.Get(0) != nil {
		postIDs = args.Get(0).([]string)
	}
	var nextCursor *time.Time
	if args.Get(1) != nil {
		nextCursor = args.Get(1).(*time.Time)
	}
	return postIDs, nextCursor, args.Error(2)
}

func (m *MockFanoutRepository) GetCelebrityPostIDs(ctx context.Context, viewerID string, cursor *time.Time, limit int) ([]string, error) {
	args := m.Called(ctx, viewerID, cursor, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

// MockSearchRepository is a mock implementation of SearchRepository
type MockSearchRepository struct {
	mock.Mock
}

func (m *MockSearchRepository) SearchPosts(ctx context.Context, filter *models.SearchFilter) ([]*models.Post, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Post), args.Error(1)
}

func (m *MockSearchRepository) SearchUsers(ctx context.Context, filter *models.SearchFilter) ([]*models.Profile, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Profile), args.Error(1)
}

func (m *MockSearchRepository) SearchBusinesses(ctx context.Context, filter *models.SearchFilter) ([]*models.BusinessProfile, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.BusinessProfile), args.Error(1)
}

func (m *MockSearchRepository) GetDiscoverPosts(ctx context.Context, lat, lng, radiusKm float64, postType *models.PostType, limit int) ([]*models.Post, error) {
	args := m.Called(ctx, lat, lng, radiusKm, postType, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Post), args.Error(1)
}

func (m *MockSearchRepository) GetDiscoverBusinesses(ctx context.Context, lat, lng, radiusKm float64, limit int) ([]*models.BusinessProfile, error) {
	args := m.Called(ctx, lat, lng, radiusKm, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.BusinessProfile), args.Error(1)
}
