package repositories

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/pkg/database"
)

// ReportRepository defines the interface for report operations
type ReportRepository interface {
	// Post reports
	CreatePostReport(ctx context.Context, report *models.PostReport) error
	GetPostReport(ctx context.Context, id string) (*models.PostReport, error)
	ListPostReports(ctx context.Context, limit, offset int) ([]*models.PostReport, int, error)
	UpdatePostReportStatus(ctx context.Context, id string, status models.ReportStatus) error

	// Comment reports
	CreateCommentReport(ctx context.Context, report *models.CommentReport) error
	GetCommentReport(ctx context.Context, id string) (*models.CommentReport, error)
	ListCommentReports(ctx context.Context, limit, offset int) ([]*models.CommentReport, int, error)
	UpdateCommentReportStatus(ctx context.Context, id string, status models.ReportStatus) error

	// User reports
	CreateUserReport(ctx context.Context, report *models.UserReport) error
	GetUserReport(ctx context.Context, id string) (*models.UserReport, error)
	ListUserReports(ctx context.Context, limit, offset int) ([]*models.UserReport, int, error)
	UpdateUserReportResolved(ctx context.Context, id string, resolved bool) error

	// Business reports
	CreateBusinessReport(ctx context.Context, report *models.BusinessReport) error
	GetBusinessReport(ctx context.Context, id string) (*models.BusinessReport, error)
	ListBusinessReports(ctx context.Context, limit, offset int) ([]*models.BusinessReport, int, error)
	UpdateBusinessReportStatus(ctx context.Context, id string, status models.ReportStatus) error
}

type reportRepository struct {
	db *database.Database
}

// NewReportRepository creates a new report repository
func NewReportRepository(db *database.Database) ReportRepository {
	return &reportRepository{db: db}
}

// Post Reports

func (r *reportRepository) CreatePostReport(ctx context.Context, report *models.PostReport) error {
	report.ID = uuid.New().String()
	report.CreatedAt = time.Now()
	report.UpdatedAt = time.Now()

	query := `
		INSERT INTO post_reports (id, user_id, post_id, reason, additional_comments, report_status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err := r.db.Pool.Exec(ctx, query,
		report.ID,
		report.UserID,
		report.PostID,
		report.Reason,
		report.AdditionalComments,
		report.ReportStatus,
		report.CreatedAt,
		report.UpdatedAt,
	)

	return err
}

func (r *reportRepository) GetPostReport(ctx context.Context, id string) (*models.PostReport, error) {
	query := `
		SELECT id, user_id, post_id, reason, additional_comments, report_status, created_at, updated_at
		FROM post_reports
		WHERE id = $1
	`

	report := &models.PostReport{}
	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&report.ID,
		&report.UserID,
		&report.PostID,
		&report.Reason,
		&report.AdditionalComments,
		&report.ReportStatus,
		&report.CreatedAt,
		&report.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return report, nil
}

func (r *reportRepository) ListPostReports(ctx context.Context, limit, offset int) ([]*models.PostReport, int, error) {
	// Get total count
	var totalCount int
	countQuery := `SELECT COUNT(*) FROM post_reports`
	err := r.db.Pool.QueryRow(ctx, countQuery).Scan(&totalCount)
	if err != nil {
		return nil, 0, err
	}

	// Get reports
	query := `
		SELECT id, user_id, post_id, reason, additional_comments, report_status, created_at, updated_at
		FROM post_reports
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := r.db.Pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var reports []*models.PostReport
	for rows.Next() {
		report := &models.PostReport{}
		err := rows.Scan(
			&report.ID,
			&report.UserID,
			&report.PostID,
			&report.Reason,
			&report.AdditionalComments,
			&report.ReportStatus,
			&report.CreatedAt,
			&report.UpdatedAt,
		)
		if err != nil {
			return nil, 0, err
		}
		reports = append(reports, report)
	}

	return reports, totalCount, nil
}

func (r *reportRepository) UpdatePostReportStatus(ctx context.Context, id string, status models.ReportStatus) error {
	query := `
		UPDATE post_reports
		SET report_status = $1, updated_at = $2
		WHERE id = $3
	`

	result, err := r.db.Pool.Exec(ctx, query, status, time.Now(), id)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

// Comment Reports

func (r *reportRepository) CreateCommentReport(ctx context.Context, report *models.CommentReport) error {
	report.ID = uuid.New().String()
	report.CreatedAt = time.Now()
	report.UpdatedAt = time.Now()

	query := `
		INSERT INTO comment_reports (id, user_id, comment_id, reason, additional_comments, report_status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err := r.db.Pool.Exec(ctx, query,
		report.ID,
		report.UserID,
		report.CommentID,
		report.Reason,
		report.AdditionalComments,
		report.ReportStatus,
		report.CreatedAt,
		report.UpdatedAt,
	)

	return err
}

func (r *reportRepository) GetCommentReport(ctx context.Context, id string) (*models.CommentReport, error) {
	query := `
		SELECT id, user_id, comment_id, reason, additional_comments, report_status, created_at, updated_at
		FROM comment_reports
		WHERE id = $1
	`

	report := &models.CommentReport{}
	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&report.ID,
		&report.UserID,
		&report.CommentID,
		&report.Reason,
		&report.AdditionalComments,
		&report.ReportStatus,
		&report.CreatedAt,
		&report.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return report, nil
}

func (r *reportRepository) ListCommentReports(ctx context.Context, limit, offset int) ([]*models.CommentReport, int, error) {
	// Get total count
	var totalCount int
	countQuery := `SELECT COUNT(*) FROM comment_reports`
	err := r.db.Pool.QueryRow(ctx, countQuery).Scan(&totalCount)
	if err != nil {
		return nil, 0, err
	}

	// Get reports
	query := `
		SELECT id, user_id, comment_id, reason, additional_comments, report_status, created_at, updated_at
		FROM comment_reports
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := r.db.Pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var reports []*models.CommentReport
	for rows.Next() {
		report := &models.CommentReport{}
		err := rows.Scan(
			&report.ID,
			&report.UserID,
			&report.CommentID,
			&report.Reason,
			&report.AdditionalComments,
			&report.ReportStatus,
			&report.CreatedAt,
			&report.UpdatedAt,
		)
		if err != nil {
			return nil, 0, err
		}
		reports = append(reports, report)
	}

	return reports, totalCount, nil
}

func (r *reportRepository) UpdateCommentReportStatus(ctx context.Context, id string, status models.ReportStatus) error {
	query := `
		UPDATE comment_reports
		SET report_status = $1, updated_at = $2
		WHERE id = $3
	`

	result, err := r.db.Pool.Exec(ctx, query, status, time.Now(), id)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

// User Reports

func (r *reportRepository) CreateUserReport(ctx context.Context, report *models.UserReport) error {
	report.ID = uuid.New().String()
	report.CreatedAt = time.Now()
	report.UpdatedAt = time.Now()

	query := `
		INSERT INTO user_reports (id, reported_user, reported_by_id, reason, description, resolved, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err := r.db.Pool.Exec(ctx, query,
		report.ID,
		report.ReportedUser,
		report.ReportedByID,
		report.Reason,
		report.Description,
		report.Resolved,
		report.CreatedAt,
		report.UpdatedAt,
	)

	return err
}

func (r *reportRepository) GetUserReport(ctx context.Context, id string) (*models.UserReport, error) {
	query := `
		SELECT id, reported_user, reported_by_id, reason, description, resolved, created_at, updated_at
		FROM user_reports
		WHERE id = $1
	`

	report := &models.UserReport{}
	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&report.ID,
		&report.ReportedUser,
		&report.ReportedByID,
		&report.Reason,
		&report.Description,
		&report.Resolved,
		&report.CreatedAt,
		&report.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return report, nil
}

func (r *reportRepository) ListUserReports(ctx context.Context, limit, offset int) ([]*models.UserReport, int, error) {
	// Get total count
	var totalCount int
	countQuery := `SELECT COUNT(*) FROM user_reports`
	err := r.db.Pool.QueryRow(ctx, countQuery).Scan(&totalCount)
	if err != nil {
		return nil, 0, err
	}

	// Get reports
	query := `
		SELECT id, reported_user, reported_by_id, reason, description, resolved, created_at, updated_at
		FROM user_reports
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := r.db.Pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var reports []*models.UserReport
	for rows.Next() {
		report := &models.UserReport{}
		err := rows.Scan(
			&report.ID,
			&report.ReportedUser,
			&report.ReportedByID,
			&report.Reason,
			&report.Description,
			&report.Resolved,
			&report.CreatedAt,
			&report.UpdatedAt,
		)
		if err != nil {
			return nil, 0, err
		}
		reports = append(reports, report)
	}

	return reports, totalCount, nil
}

func (r *reportRepository) UpdateUserReportResolved(ctx context.Context, id string, resolved bool) error {
	query := `
		UPDATE user_reports
		SET resolved = $1, updated_at = $2
		WHERE id = $3
	`

	result, err := r.db.Pool.Exec(ctx, query, resolved, time.Now(), id)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

// Business Reports

func (r *reportRepository) CreateBusinessReport(ctx context.Context, report *models.BusinessReport) error {
	report.ID = uuid.New().String()
	report.CreatedAt = time.Now()
	report.UpdatedAt = time.Now()

	query := `
		INSERT INTO business_reports (id, business_id, user_id, reason, additional_comments, report_status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err := r.db.Pool.Exec(ctx, query,
		report.ID,
		report.BusinessID,
		report.UserID,
		report.Reason,
		report.AdditionalComments,
		report.ReportStatus,
		report.CreatedAt,
		report.UpdatedAt,
	)

	return err
}

func (r *reportRepository) GetBusinessReport(ctx context.Context, id string) (*models.BusinessReport, error) {
	query := `
		SELECT id, business_id, user_id, reason, additional_comments, report_status, created_at, updated_at
		FROM business_reports
		WHERE id = $1
	`

	report := &models.BusinessReport{}
	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&report.ID,
		&report.BusinessID,
		&report.UserID,
		&report.Reason,
		&report.AdditionalComments,
		&report.ReportStatus,
		&report.CreatedAt,
		&report.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return report, nil
}

func (r *reportRepository) ListBusinessReports(ctx context.Context, limit, offset int) ([]*models.BusinessReport, int, error) {
	// Get total count
	var totalCount int
	countQuery := `SELECT COUNT(*) FROM business_reports`
	err := r.db.Pool.QueryRow(ctx, countQuery).Scan(&totalCount)
	if err != nil {
		return nil, 0, err
	}

	// Get reports
	query := `
		SELECT id, business_id, user_id, reason, additional_comments, report_status, created_at, updated_at
		FROM business_reports
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := r.db.Pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var reports []*models.BusinessReport
	for rows.Next() {
		report := &models.BusinessReport{}
		err := rows.Scan(
			&report.ID,
			&report.BusinessID,
			&report.UserID,
			&report.Reason,
			&report.AdditionalComments,
			&report.ReportStatus,
			&report.CreatedAt,
			&report.UpdatedAt,
		)
		if err != nil {
			return nil, 0, err
		}
		reports = append(reports, report)
	}

	return reports, totalCount, nil
}

func (r *reportRepository) UpdateBusinessReportStatus(ctx context.Context, id string, status models.ReportStatus) error {
	query := `
		UPDATE business_reports
		SET report_status = $1, updated_at = $2
		WHERE id = $3
	`

	result, err := r.db.Pool.Exec(ctx, query, status, time.Now(), id)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}
