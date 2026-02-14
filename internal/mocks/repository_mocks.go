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
