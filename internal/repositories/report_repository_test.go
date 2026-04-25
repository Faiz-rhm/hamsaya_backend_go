package repositories_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/internal/repositories"
	"github.com/hamsaya/backend/internal/testutil"
)

func newReportRepo(pool *testutil.MockPool) repositories.ReportRepository {
	return repositories.NewReportRepository(testutil.NewTestDB(pool))
}

// --- PostReport ---

func TestReportRepository_CreatePostReport_Success(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newReportRepo(pool)

	pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(pgconn.NewCommandTag("INSERT 1"), nil)

	report := &models.PostReport{
		UserID: "user-1", PostID: "post-1",
		Reason: "spam", ReportStatus: models.ReportStatusPending,
	}
	err := repo.CreatePostReport(context.Background(), report)
	require.NoError(t, err)
	assert.NotEmpty(t, report.ID) // repo sets UUID
}

func TestReportRepository_CreatePostReport_DBError(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newReportRepo(pool)

	pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(pgconn.NewCommandTag(""), errors.New("db error"))

	err := repo.CreatePostReport(context.Background(), &models.PostReport{
		UserID: "u1", PostID: "p1", Reason: "spam", ReportStatus: models.ReportStatusPending,
	})
	require.Error(t, err)
}

func TestReportRepository_GetPostReport_Success(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newReportRepo(pool)

	now := time.Now()
	pool.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(testutil.NewMockRow(func(dest ...any) error {
			*dest[0].(*string) = "report-1"
			*dest[1].(*string) = "user-1"
			*dest[2].(*string) = "post-1"
			*dest[3].(*string) = "spam"
			*dest[4].(**string) = nil
			*dest[5].(*models.ReportStatus) = models.ReportStatusPending
			*dest[6].(*time.Time) = now
			*dest[7].(*time.Time) = now
			return nil
		}))

	report, err := repo.GetPostReport(context.Background(), "report-1")
	require.NoError(t, err)
	assert.Equal(t, "report-1", report.ID)
	assert.Equal(t, models.ReportStatusPending, report.ReportStatus)
}

func TestReportRepository_UpdatePostReportStatus_Success(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newReportRepo(pool)

	pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(pgconn.NewCommandTag("UPDATE 1"), nil)

	err := repo.UpdatePostReportStatus(context.Background(), "report-1", models.ReportStatusResolved)
	require.NoError(t, err)
}

// --- CommentReport ---

func TestReportRepository_CreateCommentReport_Success(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newReportRepo(pool)

	pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(pgconn.NewCommandTag("INSERT 1"), nil)

	report := &models.CommentReport{
		UserID: "user-1", CommentID: "comment-1",
		Reason: "offensive", ReportStatus: models.ReportStatusPending,
	}
	err := repo.CreateCommentReport(context.Background(), report)
	require.NoError(t, err)
	assert.NotEmpty(t, report.ID)
}

func TestReportRepository_UpdateCommentReportStatus_Success(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newReportRepo(pool)

	pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(pgconn.NewCommandTag("UPDATE 1"), nil)

	err := repo.UpdateCommentReportStatus(context.Background(), "report-1", models.ReportStatusRejected)
	require.NoError(t, err)
}

// --- UserReport ---

func TestReportRepository_CreateUserReport_Success(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newReportRepo(pool)

	pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(pgconn.NewCommandTag("INSERT 1"), nil)

	report := &models.UserReport{
		ReportedUser: "bad-user", ReportedByID: "user-1", Reason: "harassment",
	}
	err := repo.CreateUserReport(context.Background(), report)
	require.NoError(t, err)
	assert.NotEmpty(t, report.ID)
}

func TestReportRepository_UpdateUserReportResolved_Success(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newReportRepo(pool)

	pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(pgconn.NewCommandTag("UPDATE 1"), nil)

	err := repo.UpdateUserReportResolved(context.Background(), "report-1", true)
	require.NoError(t, err)
}

// --- ListPostReports ---

func TestReportRepository_ListPostReports_Success(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newReportRepo(pool)

	now := time.Now()

	// COUNT query
	pool.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(testutil.NewMockRow(func(dest ...any) error {
			*dest[0].(*int) = 1
			return nil
		}))

	// List query
	pool.On("Query", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(testutil.NewFuncRows(func(dest ...any) error {
			*dest[0].(*string) = "report-1"
			*dest[1].(*string) = "user-1"
			*dest[2].(*string) = "post-1"
			*dest[3].(*string) = "spam"
			*dest[4].(**string) = nil
			*dest[5].(*models.ReportStatus) = models.ReportStatusPending
			*dest[6].(*time.Time) = now
			*dest[7].(*time.Time) = now
			return nil
		}), nil)

	reports, total, err := repo.ListPostReports(context.Background(), 10, 0)
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, reports, 1)
}
