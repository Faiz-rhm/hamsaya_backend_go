package services

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/hamsaya/backend/internal/mocks"
	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestReportService_ReportPost(t *testing.T) {
	tests := []struct {
		name          string
		userID        string
		postID        string
		request       *models.CreatePostReportRequest
		setupMocks    func(*mocks.MockReportRepository, *mocks.MockPostRepository, *mocks.MockUserRepository)
		expectedError string
	}{
		{
			name:   "successful post report",
			userID: "user-123",
			postID: "post-456",
			request: &models.CreatePostReportRequest{
				Reason:             "Spam or misleading",
				AdditionalComments: testutil.StringPtr("This contains fake info"),
			},
			setupMocks: func(reportRepo *mocks.MockReportRepository, postRepo *mocks.MockPostRepository, userRepo *mocks.MockUserRepository) {
				post := testutil.CreateTestPost("post-456", "other-user", models.PostTypeFeed)
				postRepo.On("GetByID", mock.Anything, "post-456").Return(post, nil)
				reportRepo.On("CreatePostReport", mock.Anything, mock.AnythingOfType("*models.PostReport")).Return(nil)
			},
			expectedError: "",
		},
		{
			name:   "cannot report own post",
			userID: "user-123",
			postID: "post-456",
			request: &models.CreatePostReportRequest{
				Reason: "Spam",
			},
			setupMocks: func(reportRepo *mocks.MockReportRepository, postRepo *mocks.MockPostRepository, userRepo *mocks.MockUserRepository) {
				post := testutil.CreateTestPost("post-456", "user-123", models.PostTypeFeed)
				postRepo.On("GetByID", mock.Anything, "post-456").Return(post, nil)
			},
			expectedError: "Cannot report your own post",
		},
		{
			name:   "post not found",
			userID: "user-123",
			postID: "post-999",
			request: &models.CreatePostReportRequest{
				Reason: "Spam",
			},
			setupMocks: func(reportRepo *mocks.MockReportRepository, postRepo *mocks.MockPostRepository, userRepo *mocks.MockUserRepository) {
				postRepo.On("GetByID", mock.Anything, "post-999").Return(nil, errors.New("not found"))
			},
			expectedError: "not found",
		},
		{
			name:   "validation error - empty reason",
			userID: "user-123",
			postID: "post-456",
			request: &models.CreatePostReportRequest{
				Reason: "",
			},
			setupMocks:    func(reportRepo *mocks.MockReportRepository, postRepo *mocks.MockPostRepository, userRepo *mocks.MockUserRepository) {},
			expectedError: "validation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			reportRepo := new(mocks.MockReportRepository)
			postRepo := new(mocks.MockPostRepository)
			userRepo := new(mocks.MockUserRepository)
			validator := testutil.CreateTestValidator()

			tt.setupMocks(reportRepo, postRepo, userRepo)

			service := NewReportService(reportRepo, postRepo, userRepo, validator)

			// Act
			err := service.ReportPost(context.Background(), tt.userID, tt.postID, tt.request)

			// Assert
			if tt.expectedError != "" {
				assert.Error(t, err)
				// Use case-insensitive contains for more flexible error matching
			errMsg := strings.ToLower(err.Error())
			expectedMsg := strings.ToLower(tt.expectedError)
			assert.Contains(t, errMsg, expectedMsg)
			} else {
				assert.NoError(t, err)
			}

			reportRepo.AssertExpectations(t)
			postRepo.AssertExpectations(t)
			userRepo.AssertExpectations(t)
		})
	}
}

func TestReportService_ReportUser(t *testing.T) {
	tests := []struct {
		name          string
		reporterID    string
		reportedID    string
		request       *models.CreateUserReportRequest
		setupMocks    func(*mocks.MockReportRepository, *mocks.MockUserRepository)
		expectedError string
	}{
		{
			name:       "successful user report",
			reporterID: "user-123",
			reportedID: "user-456",
			request: &models.CreateUserReportRequest{
				Reason:      "Harassment",
				Description: testutil.StringPtr("Sending abusive messages"),
			},
			setupMocks: func(reportRepo *mocks.MockReportRepository, userRepo *mocks.MockUserRepository) {
				user := testutil.CreateTestUser("user-456", "reported@example.com")
				userRepo.On("GetByID", mock.Anything, "user-456").Return(user, nil)
				reportRepo.On("CreateUserReport", mock.Anything, mock.AnythingOfType("*models.UserReport")).Return(nil)
			},
			expectedError: "",
		},
		{
			name:       "cannot report yourself",
			reporterID: "user-123",
			reportedID: "user-123",
			request: &models.CreateUserReportRequest{
				Reason: "Test",
			},
			setupMocks:    func(reportRepo *mocks.MockReportRepository, userRepo *mocks.MockUserRepository) {},
			expectedError: "Cannot report yourself",
		},
		{
			name:       "reported user not found",
			reporterID: "user-123",
			reportedID: "user-999",
			request: &models.CreateUserReportRequest{
				Reason: "Harassment",
			},
			setupMocks: func(reportRepo *mocks.MockReportRepository, userRepo *mocks.MockUserRepository) {
				userRepo.On("GetByID", mock.Anything, "user-999").Return(nil, errors.New("not found"))
			},
			expectedError: "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			reportRepo := new(mocks.MockReportRepository)
			postRepo := new(mocks.MockPostRepository)
			userRepo := new(mocks.MockUserRepository)
			validator := testutil.CreateTestValidator()

			tt.setupMocks(reportRepo, userRepo)

			service := NewReportService(reportRepo, postRepo, userRepo, validator)

			// Act
			err := service.ReportUser(context.Background(), tt.reporterID, tt.reportedID, tt.request)

			// Assert
			if tt.expectedError != "" {
				assert.Error(t, err)
				// Use case-insensitive contains for more flexible error matching
			errMsg := strings.ToLower(err.Error())
			expectedMsg := strings.ToLower(tt.expectedError)
			assert.Contains(t, errMsg, expectedMsg)
			} else {
				assert.NoError(t, err)
			}

			reportRepo.AssertExpectations(t)
			userRepo.AssertExpectations(t)
		})
	}
}

func TestReportService_ListPostReports(t *testing.T) {
	tests := []struct {
		name           string
		page           int
		limit          int
		setupMocks     func(*mocks.MockReportRepository)
		expectedCount  int
		expectedTotal  int
		expectedPage   int
		expectedLimit  int
		expectedError  string
	}{
		{
			name:  "successful list with default pagination",
			page:  1,
			limit: 20,
			setupMocks: func(reportRepo *mocks.MockReportRepository) {
				reports := []*models.PostReport{
					{ID: "report-1", PostID: "post-1", UserID: "user-1", Reason: "Spam"},
					{ID: "report-2", PostID: "post-2", UserID: "user-2", Reason: "Inappropriate"},
				}
				reportRepo.On("ListPostReports", mock.Anything, 20, 0).Return(reports, 50, nil)
			},
			expectedCount: 2,
			expectedTotal: 50,
			expectedPage:  1,
			expectedLimit: 20,
		},
		{
			name:  "pagination - page 2",
			page:  2,
			limit: 10,
			setupMocks: func(reportRepo *mocks.MockReportRepository) {
				reports := []*models.PostReport{
					{ID: "report-11", PostID: "post-11", UserID: "user-11", Reason: "Spam"},
				}
				reportRepo.On("ListPostReports", mock.Anything, 10, 10).Return(reports, 25, nil)
			},
			expectedCount: 1,
			expectedTotal: 25,
			expectedPage:  2,
			expectedLimit: 10,
		},
		{
			name:  "invalid page defaults to 1",
			page:  0,
			limit: 20,
			setupMocks: func(reportRepo *mocks.MockReportRepository) {
				reports := []*models.PostReport{}
				reportRepo.On("ListPostReports", mock.Anything, 20, 0).Return(reports, 0, nil)
			},
			expectedCount: 0,
			expectedTotal: 0,
			expectedPage:  1,
			expectedLimit: 20,
		},
		{
			name:  "limit exceeds max defaults to 20",
			page:  1,
			limit: 200,
			setupMocks: func(reportRepo *mocks.MockReportRepository) {
				reports := []*models.PostReport{}
				reportRepo.On("ListPostReports", mock.Anything, 20, 0).Return(reports, 0, nil)
			},
			expectedCount: 0,
			expectedTotal: 0,
			expectedPage:  1,
			expectedLimit: 20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			reportRepo := new(mocks.MockReportRepository)
			postRepo := new(mocks.MockPostRepository)
			userRepo := new(mocks.MockUserRepository)
			validator := testutil.CreateTestValidator()

			tt.setupMocks(reportRepo)

			service := NewReportService(reportRepo, postRepo, userRepo, validator)

			// Act
			result, err := service.ListPostReports(context.Background(), tt.page, tt.limit)

			// Assert
			if tt.expectedError != "" {
				assert.Error(t, err)
				// Use case-insensitive contains for more flexible error matching
			errMsg := strings.ToLower(err.Error())
			expectedMsg := strings.ToLower(tt.expectedError)
			assert.Contains(t, errMsg, expectedMsg)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.expectedTotal, result.TotalCount)
				assert.Equal(t, tt.expectedPage, result.Page)
				assert.Equal(t, tt.expectedLimit, result.Limit)

				if reports, ok := result.Reports.([]*models.PostReport); ok {
					assert.Equal(t, tt.expectedCount, len(reports))
				}
			}

			reportRepo.AssertExpectations(t)
		})
	}
}

func TestReportService_UpdatePostReportStatus(t *testing.T) {
	tests := []struct {
		name          string
		reportID      string
		status        models.ReportStatus
		setupMocks    func(*mocks.MockReportRepository)
		expectedError string
	}{
		{
			name:     "successful status update to RESOLVED",
			reportID: "report-123",
			status:   models.ReportStatusResolved,
			setupMocks: func(reportRepo *mocks.MockReportRepository) {
				reportRepo.On("UpdatePostReportStatus", mock.Anything, "report-123", models.ReportStatusResolved).Return(nil)
			},
			expectedError: "",
		},
		{
			name:     "successful status update to REVIEWING",
			reportID: "report-123",
			status:   models.ReportStatusReviewing,
			setupMocks: func(reportRepo *mocks.MockReportRepository) {
				reportRepo.On("UpdatePostReportStatus", mock.Anything, "report-123", models.ReportStatusReviewing).Return(nil)
			},
			expectedError: "",
		},
		{
			name:     "successful status update to REJECTED",
			reportID: "report-123",
			status:   models.ReportStatusRejected,
			setupMocks: func(reportRepo *mocks.MockReportRepository) {
				reportRepo.On("UpdatePostReportStatus", mock.Anything, "report-123", models.ReportStatusRejected).Return(nil)
			},
			expectedError: "",
		},
		{
			name:          "invalid status",
			reportID:      "report-123",
			status:        "INVALID",
			setupMocks:    func(reportRepo *mocks.MockReportRepository) {},
			expectedError: "Invalid report status",
		},
		{
			name:     "report not found",
			reportID: "report-999",
			status:   models.ReportStatusResolved,
			setupMocks: func(reportRepo *mocks.MockReportRepository) {
				reportRepo.On("UpdatePostReportStatus", mock.Anything, "report-999", models.ReportStatusResolved).Return(errors.New("report not found"))
			},
			expectedError: "Report not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			reportRepo := new(mocks.MockReportRepository)
			postRepo := new(mocks.MockPostRepository)
			userRepo := new(mocks.MockUserRepository)
			validator := testutil.CreateTestValidator()

			tt.setupMocks(reportRepo)

			service := NewReportService(reportRepo, postRepo, userRepo, validator)

			// Act
			err := service.UpdatePostReportStatus(context.Background(), tt.reportID, tt.status)

			// Assert
			if tt.expectedError != "" {
				assert.Error(t, err)
				// Use case-insensitive contains for more flexible error matching
			errMsg := strings.ToLower(err.Error())
			expectedMsg := strings.ToLower(tt.expectedError)
			assert.Contains(t, errMsg, expectedMsg)
			} else {
				assert.NoError(t, err)
			}

			reportRepo.AssertExpectations(t)
		})
	}
}

func TestReportService_GetPostReport(t *testing.T) {
	tests := []struct {
		name          string
		reportID      string
		setupMocks    func(*mocks.MockReportRepository)
		expectedError string
	}{
		{
			name:     "successful get report",
			reportID: "report-123",
			setupMocks: func(reportRepo *mocks.MockReportRepository) {
				report := &models.PostReport{
					ID:           "report-123",
					PostID:       "post-456",
					UserID:       "user-789",
					Reason:       "Spam",
					ReportStatus: models.ReportStatusPending,
				}
				reportRepo.On("GetPostReport", mock.Anything, "report-123").Return(report, nil)
			},
			expectedError: "",
		},
		{
			name:     "report not found",
			reportID: "report-999",
			setupMocks: func(reportRepo *mocks.MockReportRepository) {
				reportRepo.On("GetPostReport", mock.Anything, "report-999").Return(nil, errors.New("not found"))
			},
			expectedError: "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			reportRepo := new(mocks.MockReportRepository)
			postRepo := new(mocks.MockPostRepository)
			userRepo := new(mocks.MockUserRepository)
			validator := testutil.CreateTestValidator()

			tt.setupMocks(reportRepo)

			service := NewReportService(reportRepo, postRepo, userRepo, validator)

			// Act
			report, err := service.GetPostReport(context.Background(), tt.reportID)

			// Assert
			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Nil(t, report)
				// Use case-insensitive contains for more flexible error matching
			errMsg := strings.ToLower(err.Error())
			expectedMsg := strings.ToLower(tt.expectedError)
			assert.Contains(t, errMsg, expectedMsg)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, report)
				assert.Equal(t, tt.reportID, report.ID)
			}

			reportRepo.AssertExpectations(t)
		})
	}
}

func TestReportService_ReportComment(t *testing.T) {
	tests := []struct {
		name          string
		userID        string
		commentID     string
		request       *models.CreateCommentReportRequest
		setupMocks    func(*mocks.MockReportRepository)
		expectedError string
	}{
		{
			name:      "successful comment report",
			userID:    "user-123",
			commentID: "comment-456",
			request: &models.CreateCommentReportRequest{
				Reason:             "Spam or misleading",
				AdditionalComments: testutil.StringPtr("This comment is inappropriate"),
			},
			setupMocks: func(reportRepo *mocks.MockReportRepository) {
				reportRepo.On("CreateCommentReport", mock.Anything, mock.AnythingOfType("*models.CommentReport")).Return(nil)
			},
			expectedError: "",
		},
		{
			name:      "validation error - empty reason",
			userID:    "user-123",
			commentID: "comment-456",
			request: &models.CreateCommentReportRequest{
				Reason: "",
			},
			setupMocks:    func(reportRepo *mocks.MockReportRepository) {},
			expectedError: "validation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			reportRepo := new(mocks.MockReportRepository)
			postRepo := new(mocks.MockPostRepository)
			userRepo := new(mocks.MockUserRepository)
			validator := testutil.CreateTestValidator()

			tt.setupMocks(reportRepo)

			service := NewReportService(reportRepo, postRepo, userRepo, validator)

			// Act
			err := service.ReportComment(context.Background(), tt.userID, tt.commentID, tt.request)

			// Assert
			if tt.expectedError != "" {
				assert.Error(t, err)
				errMsg := strings.ToLower(err.Error())
				expectedMsg := strings.ToLower(tt.expectedError)
				assert.Contains(t, errMsg, expectedMsg)
			} else {
				assert.NoError(t, err)
			}

			reportRepo.AssertExpectations(t)
		})
	}
}

func TestReportService_ReportBusiness(t *testing.T) {
	tests := []struct {
		name          string
		userID        string
		businessID    string
		request       *models.CreateBusinessReportRequest
		setupMocks    func(*mocks.MockReportRepository)
		expectedError string
	}{
		{
			name:       "successful business report",
			userID:     "user-123",
			businessID: "business-456",
			request: &models.CreateBusinessReportRequest{
				Reason:             "Fake business",
				AdditionalComments: testutil.StringPtr("This business is not legitimate"),
			},
			setupMocks: func(reportRepo *mocks.MockReportRepository) {
				reportRepo.On("CreateBusinessReport", mock.Anything, mock.AnythingOfType("*models.BusinessReport")).Return(nil)
			},
			expectedError: "",
		},
		{
			name:       "validation error - empty reason",
			userID:     "user-123",
			businessID: "business-456",
			request: &models.CreateBusinessReportRequest{
				Reason: "",
			},
			setupMocks:    func(reportRepo *mocks.MockReportRepository) {},
			expectedError: "validation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			reportRepo := new(mocks.MockReportRepository)
			postRepo := new(mocks.MockPostRepository)
			userRepo := new(mocks.MockUserRepository)
			validator := testutil.CreateTestValidator()

			tt.setupMocks(reportRepo)

			service := NewReportService(reportRepo, postRepo, userRepo, validator)

			// Act
			err := service.ReportBusiness(context.Background(), tt.userID, tt.businessID, tt.request)

			// Assert
			if tt.expectedError != "" {
				assert.Error(t, err)
				errMsg := strings.ToLower(err.Error())
				expectedMsg := strings.ToLower(tt.expectedError)
				assert.Contains(t, errMsg, expectedMsg)
			} else {
				assert.NoError(t, err)
			}

			reportRepo.AssertExpectations(t)
		})
	}
}

func TestReportService_ListCommentReports(t *testing.T) {
	tests := []struct {
		name          string
		page          int
		limit         int
		setupMocks    func(*mocks.MockReportRepository)
		expectedCount int
		expectedTotal int
		expectedPage  int
		expectedLimit int
	}{
		{
			name:  "successful list with default pagination",
			page:  1,
			limit: 20,
			setupMocks: func(reportRepo *mocks.MockReportRepository) {
				reports := []*models.CommentReport{
					{ID: "report-1", CommentID: "comment-1", UserID: "user-1", Reason: "Spam"},
					{ID: "report-2", CommentID: "comment-2", UserID: "user-2", Reason: "Offensive"},
				}
				reportRepo.On("ListCommentReports", mock.Anything, 20, 0).Return(reports, 35, nil)
			},
			expectedCount: 2,
			expectedTotal: 35,
			expectedPage:  1,
			expectedLimit: 20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			reportRepo := new(mocks.MockReportRepository)
			postRepo := new(mocks.MockPostRepository)
			userRepo := new(mocks.MockUserRepository)
			validator := testutil.CreateTestValidator()

			tt.setupMocks(reportRepo)

			service := NewReportService(reportRepo, postRepo, userRepo, validator)

			// Act
			result, err := service.ListCommentReports(context.Background(), tt.page, tt.limit)

			// Assert
			assert.NoError(t, err)
			assert.NotNil(t, result)
			assert.Equal(t, tt.expectedTotal, result.TotalCount)
			assert.Equal(t, tt.expectedPage, result.Page)
			assert.Equal(t, tt.expectedLimit, result.Limit)

			if reports, ok := result.Reports.([]*models.CommentReport); ok {
				assert.Equal(t, tt.expectedCount, len(reports))
			}

			reportRepo.AssertExpectations(t)
		})
	}
}

func TestReportService_ListUserReports(t *testing.T) {
	tests := []struct {
		name          string
		page          int
		limit         int
		setupMocks    func(*mocks.MockReportRepository)
		expectedCount int
		expectedTotal int
		expectedPage  int
		expectedLimit int
	}{
		{
			name:  "successful list with default pagination",
			page:  1,
			limit: 20,
			setupMocks: func(reportRepo *mocks.MockReportRepository) {
				reports := []*models.UserReport{
					{ID: "report-1", ReportedUser: "user-1", ReportedByID: "user-2", Reason: "Harassment"},
					{ID: "report-2", ReportedUser: "user-3", ReportedByID: "user-4", Reason: "Spam"},
				}
				reportRepo.On("ListUserReports", mock.Anything, 20, 0).Return(reports, 42, nil)
			},
			expectedCount: 2,
			expectedTotal: 42,
			expectedPage:  1,
			expectedLimit: 20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			reportRepo := new(mocks.MockReportRepository)
			postRepo := new(mocks.MockPostRepository)
			userRepo := new(mocks.MockUserRepository)
			validator := testutil.CreateTestValidator()

			tt.setupMocks(reportRepo)

			service := NewReportService(reportRepo, postRepo, userRepo, validator)

			// Act
			result, err := service.ListUserReports(context.Background(), tt.page, tt.limit)

			// Assert
			assert.NoError(t, err)
			assert.NotNil(t, result)
			assert.Equal(t, tt.expectedTotal, result.TotalCount)
			assert.Equal(t, tt.expectedPage, result.Page)
			assert.Equal(t, tt.expectedLimit, result.Limit)

			if reports, ok := result.Reports.([]*models.UserReport); ok {
				assert.Equal(t, tt.expectedCount, len(reports))
			}

			reportRepo.AssertExpectations(t)
		})
	}
}

func TestReportService_ListBusinessReports(t *testing.T) {
	tests := []struct {
		name          string
		page          int
		limit         int
		setupMocks    func(*mocks.MockReportRepository)
		expectedCount int
		expectedTotal int
		expectedPage  int
		expectedLimit int
	}{
		{
			name:  "successful list with default pagination",
			page:  1,
			limit: 20,
			setupMocks: func(reportRepo *mocks.MockReportRepository) {
				reports := []*models.BusinessReport{
					{ID: "report-1", BusinessID: "business-1", UserID: "user-1", Reason: "Fake"},
					{ID: "report-2", BusinessID: "business-2", UserID: "user-2", Reason: "Scam"},
				}
				reportRepo.On("ListBusinessReports", mock.Anything, 20, 0).Return(reports, 28, nil)
			},
			expectedCount: 2,
			expectedTotal: 28,
			expectedPage:  1,
			expectedLimit: 20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			reportRepo := new(mocks.MockReportRepository)
			postRepo := new(mocks.MockPostRepository)
			userRepo := new(mocks.MockUserRepository)
			validator := testutil.CreateTestValidator()

			tt.setupMocks(reportRepo)

			service := NewReportService(reportRepo, postRepo, userRepo, validator)

			// Act
			result, err := service.ListBusinessReports(context.Background(), tt.page, tt.limit)

			// Assert
			assert.NoError(t, err)
			assert.NotNil(t, result)
			assert.Equal(t, tt.expectedTotal, result.TotalCount)
			assert.Equal(t, tt.expectedPage, result.Page)
			assert.Equal(t, tt.expectedLimit, result.Limit)

			if reports, ok := result.Reports.([]*models.BusinessReport); ok {
				assert.Equal(t, tt.expectedCount, len(reports))
			}

			reportRepo.AssertExpectations(t)
		})
	}
}

func TestReportService_UpdateCommentReportStatus(t *testing.T) {
	tests := []struct {
		name          string
		reportID      string
		status        models.ReportStatus
		setupMocks    func(*mocks.MockReportRepository)
		expectedError string
	}{
		{
			name:     "successful status update to RESOLVED",
			reportID: "report-123",
			status:   models.ReportStatusResolved,
			setupMocks: func(reportRepo *mocks.MockReportRepository) {
				reportRepo.On("UpdateCommentReportStatus", mock.Anything, "report-123", models.ReportStatusResolved).Return(nil)
			},
			expectedError: "",
		},
		{
			name:          "invalid status",
			reportID:      "report-123",
			status:        "INVALID",
			setupMocks:    func(reportRepo *mocks.MockReportRepository) {},
			expectedError: "Invalid report status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			reportRepo := new(mocks.MockReportRepository)
			postRepo := new(mocks.MockPostRepository)
			userRepo := new(mocks.MockUserRepository)
			validator := testutil.CreateTestValidator()

			tt.setupMocks(reportRepo)

			service := NewReportService(reportRepo, postRepo, userRepo, validator)

			// Act
			err := service.UpdateCommentReportStatus(context.Background(), tt.reportID, tt.status)

			// Assert
			if tt.expectedError != "" {
				assert.Error(t, err)
				errMsg := strings.ToLower(err.Error())
				expectedMsg := strings.ToLower(tt.expectedError)
				assert.Contains(t, errMsg, expectedMsg)
			} else {
				assert.NoError(t, err)
			}

			reportRepo.AssertExpectations(t)
		})
	}
}

func TestReportService_UpdateUserReportStatus(t *testing.T) {
	tests := []struct {
		name          string
		reportID      string
		resolved      bool
		setupMocks    func(*mocks.MockReportRepository)
		expectedError string
	}{
		{
			name:     "successful status update to resolved",
			reportID: "report-123",
			resolved: true,
			setupMocks: func(reportRepo *mocks.MockReportRepository) {
				reportRepo.On("UpdateUserReportResolved", mock.Anything, "report-123", true).Return(nil)
			},
			expectedError: "",
		},
		{
			name:     "successful status update to not resolved",
			reportID: "report-123",
			resolved: false,
			setupMocks: func(reportRepo *mocks.MockReportRepository) {
				reportRepo.On("UpdateUserReportResolved", mock.Anything, "report-123", false).Return(nil)
			},
			expectedError: "",
		},
		{
			name:     "report not found",
			reportID: "report-999",
			resolved: true,
			setupMocks: func(reportRepo *mocks.MockReportRepository) {
				reportRepo.On("UpdateUserReportResolved", mock.Anything, "report-999", true).Return(errors.New("report not found"))
			},
			expectedError: "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			reportRepo := new(mocks.MockReportRepository)
			postRepo := new(mocks.MockPostRepository)
			userRepo := new(mocks.MockUserRepository)
			validator := testutil.CreateTestValidator()

			tt.setupMocks(reportRepo)

			service := NewReportService(reportRepo, postRepo, userRepo, validator)

			// Act
			err := service.UpdateUserReportStatus(context.Background(), tt.reportID, tt.resolved)

			// Assert
			if tt.expectedError != "" {
				assert.Error(t, err)
				errMsg := strings.ToLower(err.Error())
				expectedMsg := strings.ToLower(tt.expectedError)
				assert.Contains(t, errMsg, expectedMsg)
			} else {
				assert.NoError(t, err)
			}

			reportRepo.AssertExpectations(t)
		})
	}
}

func TestReportService_UpdateBusinessReportStatus(t *testing.T) {
	tests := []struct {
		name          string
		reportID      string
		status        models.ReportStatus
		setupMocks    func(*mocks.MockReportRepository)
		expectedError string
	}{
		{
			name:     "successful status update to RESOLVED",
			reportID: "report-123",
			status:   models.ReportStatusResolved,
			setupMocks: func(reportRepo *mocks.MockReportRepository) {
				reportRepo.On("UpdateBusinessReportStatus", mock.Anything, "report-123", models.ReportStatusResolved).Return(nil)
			},
			expectedError: "",
		},
		{
			name:          "invalid status",
			reportID:      "report-123",
			status:        "INVALID",
			setupMocks:    func(reportRepo *mocks.MockReportRepository) {},
			expectedError: "Invalid report status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			reportRepo := new(mocks.MockReportRepository)
			postRepo := new(mocks.MockPostRepository)
			userRepo := new(mocks.MockUserRepository)
			validator := testutil.CreateTestValidator()

			tt.setupMocks(reportRepo)

			service := NewReportService(reportRepo, postRepo, userRepo, validator)

			// Act
			err := service.UpdateBusinessReportStatus(context.Background(), tt.reportID, tt.status)

			// Assert
			if tt.expectedError != "" {
				assert.Error(t, err)
				errMsg := strings.ToLower(err.Error())
				expectedMsg := strings.ToLower(tt.expectedError)
				assert.Contains(t, errMsg, expectedMsg)
			} else {
				assert.NoError(t, err)
			}

			reportRepo.AssertExpectations(t)
		})
	}
}

func TestReportService_GetCommentReport(t *testing.T) {
	tests := []struct {
		name          string
		reportID      string
		setupMocks    func(*mocks.MockReportRepository)
		expectedError string
	}{
		{
			name:     "successful get report",
			reportID: "report-123",
			setupMocks: func(reportRepo *mocks.MockReportRepository) {
				report := &models.CommentReport{
					ID:           "report-123",
					CommentID:    "comment-456",
					UserID:       "user-789",
					Reason:       "Offensive",
					ReportStatus: models.ReportStatusPending,
				}
				reportRepo.On("GetCommentReport", mock.Anything, "report-123").Return(report, nil)
			},
			expectedError: "",
		},
		{
			name:     "report not found",
			reportID: "report-999",
			setupMocks: func(reportRepo *mocks.MockReportRepository) {
				reportRepo.On("GetCommentReport", mock.Anything, "report-999").Return(nil, errors.New("not found"))
			},
			expectedError: "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			reportRepo := new(mocks.MockReportRepository)
			postRepo := new(mocks.MockPostRepository)
			userRepo := new(mocks.MockUserRepository)
			validator := testutil.CreateTestValidator()

			tt.setupMocks(reportRepo)

			service := NewReportService(reportRepo, postRepo, userRepo, validator)

			// Act
			report, err := service.GetCommentReport(context.Background(), tt.reportID)

			// Assert
			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Nil(t, report)
				errMsg := strings.ToLower(err.Error())
				expectedMsg := strings.ToLower(tt.expectedError)
				assert.Contains(t, errMsg, expectedMsg)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, report)
				assert.Equal(t, tt.reportID, report.ID)
			}

			reportRepo.AssertExpectations(t)
		})
	}
}

func TestReportService_GetUserReport(t *testing.T) {
	tests := []struct {
		name          string
		reportID      string
		setupMocks    func(*mocks.MockReportRepository)
		expectedError string
	}{
		{
			name:     "successful get report",
			reportID: "report-123",
			setupMocks: func(reportRepo *mocks.MockReportRepository) {
				report := &models.UserReport{
					ID:           "report-123",
					ReportedUser: "user-456",
					ReportedByID: "user-789",
					Reason:       "Harassment",
					Resolved:     false,
				}
				reportRepo.On("GetUserReport", mock.Anything, "report-123").Return(report, nil)
			},
			expectedError: "",
		},
		{
			name:     "report not found",
			reportID: "report-999",
			setupMocks: func(reportRepo *mocks.MockReportRepository) {
				reportRepo.On("GetUserReport", mock.Anything, "report-999").Return(nil, errors.New("not found"))
			},
			expectedError: "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			reportRepo := new(mocks.MockReportRepository)
			postRepo := new(mocks.MockPostRepository)
			userRepo := new(mocks.MockUserRepository)
			validator := testutil.CreateTestValidator()

			tt.setupMocks(reportRepo)

			service := NewReportService(reportRepo, postRepo, userRepo, validator)

			// Act
			report, err := service.GetUserReport(context.Background(), tt.reportID)

			// Assert
			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Nil(t, report)
				errMsg := strings.ToLower(err.Error())
				expectedMsg := strings.ToLower(tt.expectedError)
				assert.Contains(t, errMsg, expectedMsg)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, report)
				assert.Equal(t, tt.reportID, report.ID)
			}

			reportRepo.AssertExpectations(t)
		})
	}
}

func TestReportService_GetBusinessReport(t *testing.T) {
	tests := []struct {
		name          string
		reportID      string
		setupMocks    func(*mocks.MockReportRepository)
		expectedError string
	}{
		{
			name:     "successful get report",
			reportID: "report-123",
			setupMocks: func(reportRepo *mocks.MockReportRepository) {
				report := &models.BusinessReport{
					ID:           "report-123",
					BusinessID:   "business-456",
					UserID:       "user-789",
					Reason:       "Fake business",
					ReportStatus: models.ReportStatusPending,
				}
				reportRepo.On("GetBusinessReport", mock.Anything, "report-123").Return(report, nil)
			},
			expectedError: "",
		},
		{
			name:     "report not found",
			reportID: "report-999",
			setupMocks: func(reportRepo *mocks.MockReportRepository) {
				reportRepo.On("GetBusinessReport", mock.Anything, "report-999").Return(nil, errors.New("not found"))
			},
			expectedError: "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			reportRepo := new(mocks.MockReportRepository)
			postRepo := new(mocks.MockPostRepository)
			userRepo := new(mocks.MockUserRepository)
			validator := testutil.CreateTestValidator()

			tt.setupMocks(reportRepo)

			service := NewReportService(reportRepo, postRepo, userRepo, validator)

			// Act
			report, err := service.GetBusinessReport(context.Background(), tt.reportID)

			// Assert
			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Nil(t, report)
				errMsg := strings.ToLower(err.Error())
				expectedMsg := strings.ToLower(tt.expectedError)
				assert.Contains(t, errMsg, expectedMsg)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, report)
				assert.Equal(t, tt.reportID, report.ID)
			}

			reportRepo.AssertExpectations(t)
		})
	}
}
