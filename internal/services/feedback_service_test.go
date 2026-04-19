package services

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/hamsaya/backend/internal/mocks"
	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestFeedbackService_SubmitFeedback(t *testing.T) {
	tests := []struct {
		name        string
		userID      string
		req         *models.CreateFeedbackRequest
		setupMocks  func(feedbackRepo *mocks.MockFeedbackRepository)
		wantErr     bool
		wantErrStr  string
		wantResp    bool
	}{
		{
			name:   "validation error - empty message",
			userID: "user-1",
			req: &models.CreateFeedbackRequest{
				Rating:  models.FeedbackRatingGood,
				Type:    models.FeedbackTypeGeneral,
				Message: "", // required field empty
			},
			setupMocks: func(feedbackRepo *mocks.MockFeedbackRepository) {
				// no repo calls expected on validation failure
			},
			wantErr:    true,
			wantErrStr: "invalid request",
		},
		{
			name:   "validation error - invalid rating",
			userID: "user-1",
			req: &models.CreateFeedbackRequest{
				Rating:  0, // out of range (must be 1-5)
				Type:    models.FeedbackTypeGeneral,
				Message: "Some feedback message",
			},
			setupMocks: func(feedbackRepo *mocks.MockFeedbackRepository) {
				// no repo calls expected on validation failure
			},
			wantErr:    true,
			wantErrStr: "invalid request",
		},
		{
			name:   "validation error - invalid type",
			userID: "user-1",
			req: &models.CreateFeedbackRequest{
				Rating:  models.FeedbackRatingGood,
				Type:    "INVALID_TYPE",
				Message: "Some feedback message",
			},
			setupMocks: func(feedbackRepo *mocks.MockFeedbackRepository) {
				// no repo calls expected on validation failure
			},
			wantErr:    true,
			wantErrStr: "invalid request",
		},
		{
			name:   "repo error - create fails",
			userID: "user-1",
			req: &models.CreateFeedbackRequest{
				Rating:  models.FeedbackRatingGood,
				Type:    models.FeedbackTypeGeneral,
				Message: "Great app, love it!",
			},
			setupMocks: func(feedbackRepo *mocks.MockFeedbackRepository) {
				feedbackRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.Feedback")).
					Return(errors.New("db error"))
			},
			wantErr:    true,
			wantErrStr: "failed to submit feedback",
		},
		{
			name:   "success",
			userID: "user-1",
			req: &models.CreateFeedbackRequest{
				Rating:  models.FeedbackRatingExcellent,
				Type:    models.FeedbackTypeBug,
				Message: "Found a bug in the app",
			},
			setupMocks: func(feedbackRepo *mocks.MockFeedbackRepository) {
				feedbackRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.Feedback")).
					Return(nil)
			},
			wantErr:  false,
			wantResp: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			feedbackRepo := &mocks.MockFeedbackRepository{}
			tt.setupMocks(feedbackRepo)

			validator := testutil.CreateTestValidator()
			svc := NewFeedbackService(feedbackRepo, validator)

			resp, err := svc.SubmitFeedback(context.Background(), tt.userID, tt.req)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, strings.ToLower(err.Error()), tt.wantErrStr)
				assert.Nil(t, resp)
			} else {
				require.NoError(t, err)
				require.NotNil(t, resp)
				assert.Equal(t, "Thank you for your feedback!", resp.Message)
			}

			feedbackRepo.AssertExpectations(t)
		})
	}
}

func TestFeedbackService_GetFeedbackStatus(t *testing.T) {
	recentTime := time.Now().AddDate(0, 0, -5)   // 5 days ago — within 30-day window
	oldTime := time.Now().AddDate(0, 0, -35)     // 35 days ago — outside 30-day window

	tests := []struct {
		name           string
		userID         string
		setupMocks     func(feedbackRepo *mocks.MockFeedbackRepository)
		wantErr        bool
		wantErrStr     string
		wantHasSubmitted bool
		wantLastFeedback bool
	}{
		{
			name:   "repo error",
			userID: "user-1",
			setupMocks: func(feedbackRepo *mocks.MockFeedbackRepository) {
				feedbackRepo.On("GetUserFeedbackStatus", mock.Anything, "user-1").
					Return(false, (*time.Time)(nil), errors.New("db error"))
			},
			wantErr:    true,
			wantErrStr: "failed to get feedback status",
		},
		{
			name:   "user has no feedback",
			userID: "user-2",
			setupMocks: func(feedbackRepo *mocks.MockFeedbackRepository) {
				feedbackRepo.On("GetUserFeedbackStatus", mock.Anything, "user-2").
					Return(false, (*time.Time)(nil), nil)
			},
			wantErr:          false,
			wantHasSubmitted: false,
			wantLastFeedback: false,
		},
		{
			name:   "user has recent feedback - within 30 days",
			userID: "user-3",
			setupMocks: func(feedbackRepo *mocks.MockFeedbackRepository) {
				feedbackRepo.On("GetUserFeedbackStatus", mock.Anything, "user-3").
					Return(true, &recentTime, nil)
			},
			wantErr:          false,
			wantHasSubmitted: true,
			wantLastFeedback: true,
		},
		{
			name:   "user has old feedback - older than 30 days resets hasSubmitted to false",
			userID: "user-4",
			setupMocks: func(feedbackRepo *mocks.MockFeedbackRepository) {
				feedbackRepo.On("GetUserFeedbackStatus", mock.Anything, "user-4").
					Return(true, &oldTime, nil)
			},
			wantErr:          false,
			wantHasSubmitted: false, // reset because feedback is older than 30 days
			wantLastFeedback: true,  // lastFeedback is still returned
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			feedbackRepo := &mocks.MockFeedbackRepository{}
			tt.setupMocks(feedbackRepo)

			validator := testutil.CreateTestValidator()
			svc := NewFeedbackService(feedbackRepo, validator)

			resp, err := svc.GetFeedbackStatus(context.Background(), tt.userID)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, strings.ToLower(err.Error()), tt.wantErrStr)
				assert.Nil(t, resp)
			} else {
				require.NoError(t, err)
				require.NotNil(t, resp)
				assert.Equal(t, tt.wantHasSubmitted, resp.HasSubmitted)
				if tt.wantLastFeedback {
					assert.NotNil(t, resp.LastFeedback)
				} else {
					assert.Nil(t, resp.LastFeedback)
				}
			}

			feedbackRepo.AssertExpectations(t)
		})
	}
}
