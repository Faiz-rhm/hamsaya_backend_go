package repositories

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/pkg/database"
	"github.com/jackc/pgx/v5"
)

// MFARepository defines the interface for MFA data operations
type MFARepository interface {
	// MFA Factor operations
	CreateFactor(ctx context.Context, factor *models.MFAFactor) error
	GetFactorByID(ctx context.Context, factorID string) (*models.MFAFactor, error)
	GetFactorsByUserID(ctx context.Context, userID string) ([]*models.MFAFactor, error)
	UpdateFactorStatus(ctx context.Context, factorID, status string) error
	DeleteFactor(ctx context.Context, factorID string) error

	// Backup Code operations
	CreateBackupCodes(ctx context.Context, codes []*models.BackupCode) error
	GetBackupCode(ctx context.Context, userID, code string) (*models.BackupCode, error)
	MarkBackupCodeAsUsed(ctx context.Context, codeID string) error
	GetUnusedBackupCodesCount(ctx context.Context, userID string) (int, error)
	DeleteAllBackupCodes(ctx context.Context, userID string) error
}

type mfaRepository struct {
	db *database.DB
}

// NewMFARepository creates a new MFA repository
func NewMFARepository(db *database.DB) MFARepository {
	return &mfaRepository{db: db}
}

// CreateFactor creates a new MFA factor
func (r *mfaRepository) CreateFactor(ctx context.Context, factor *models.MFAFactor) error {
	query := `
		INSERT INTO mfa_factors (id, user_id, factor_type, secret_key, factor_id, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err := r.db.Pool.Exec(ctx, query,
		factor.ID,
		factor.UserID,
		factor.Type,
		factor.SecretKey,
		factor.FactorID,
		factor.Status,
		factor.CreatedAt,
		factor.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create MFA factor: %w", err)
	}

	return nil
}

// GetFactorByID retrieves an MFA factor by ID
func (r *mfaRepository) GetFactorByID(ctx context.Context, factorID string) (*models.MFAFactor, error) {
	query := `
		SELECT id, user_id, factor_type, secret_key, factor_id, status, created_at, updated_at, deleted_at
		FROM mfa_factors
		WHERE id = $1 AND deleted_at IS NULL
	`

	factor := &models.MFAFactor{}
	err := r.db.Pool.QueryRow(ctx, query, factorID).Scan(
		&factor.ID,
		&factor.UserID,
		&factor.Type,
		&factor.SecretKey,
		&factor.FactorID,
		&factor.Status,
		&factor.CreatedAt,
		&factor.UpdatedAt,
		&factor.DeletedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("MFA factor not found")
		}
		return nil, fmt.Errorf("failed to get MFA factor: %w", err)
	}

	return factor, nil
}

// GetFactorsByUserID retrieves all MFA factors for a user
func (r *mfaRepository) GetFactorsByUserID(ctx context.Context, userID string) ([]*models.MFAFactor, error) {
	query := `
		SELECT id, user_id, factor_type, secret_key, factor_id, status, created_at, updated_at, deleted_at
		FROM mfa_factors
		WHERE user_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC
	`

	rows, err := r.db.Pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get MFA factors: %w", err)
	}
	defer rows.Close()

	var factors []*models.MFAFactor
	for rows.Next() {
		factor := &models.MFAFactor{}
		err := rows.Scan(
			&factor.ID,
			&factor.UserID,
			&factor.Type,
			&factor.SecretKey,
			&factor.FactorID,
			&factor.Status,
			&factor.CreatedAt,
			&factor.UpdatedAt,
			&factor.DeletedAt,
		)
		if err != nil {
			return nil, err
		}
		factors = append(factors, factor)
	}

	return factors, rows.Err()
}

// UpdateFactorStatus updates the status of an MFA factor
func (r *mfaRepository) UpdateFactorStatus(ctx context.Context, factorID, status string) error {
	query := `
		UPDATE mfa_factors
		SET status = $2, updated_at = $3
		WHERE id = $1 AND deleted_at IS NULL
	`

	result, err := r.db.Pool.Exec(ctx, query, factorID, status, time.Now())
	if err != nil {
		return fmt.Errorf("failed to update MFA factor status: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("MFA factor not found")
	}

	return nil
}

// DeleteFactor soft deletes an MFA factor
func (r *mfaRepository) DeleteFactor(ctx context.Context, factorID string) error {
	query := `
		UPDATE mfa_factors
		SET deleted_at = $2, updated_at = $2
		WHERE id = $1 AND deleted_at IS NULL
	`

	result, err := r.db.Pool.Exec(ctx, query, factorID, time.Now())
	if err != nil {
		return fmt.Errorf("failed to delete MFA factor: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("MFA factor not found")
	}

	return nil
}

// CreateBackupCodes creates multiple backup codes
func (r *mfaRepository) CreateBackupCodes(ctx context.Context, codes []*models.BackupCode) error {
	// Use a transaction to ensure all codes are created together
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	query := `
		INSERT INTO mfa_backup_codes (id, user_id, code, used, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`

	for _, code := range codes {
		_, err := tx.Exec(ctx, query,
			code.ID,
			code.UserID,
			code.Code,
			code.Used,
			code.CreatedAt,
		)
		if err != nil {
			return fmt.Errorf("failed to create backup code: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetBackupCode retrieves a backup code for a user
func (r *mfaRepository) GetBackupCode(ctx context.Context, userID, code string) (*models.BackupCode, error) {
	query := `
		SELECT id, user_id, code, used, used_at, created_at
		FROM mfa_backup_codes
		WHERE user_id = $1 AND code = $2
	`

	backupCode := &models.BackupCode{}
	err := r.db.Pool.QueryRow(ctx, query, userID, code).Scan(
		&backupCode.ID,
		&backupCode.UserID,
		&backupCode.Code,
		&backupCode.Used,
		&backupCode.UsedAt,
		&backupCode.CreatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("backup code not found")
		}
		return nil, fmt.Errorf("failed to get backup code: %w", err)
	}

	return backupCode, nil
}

// MarkBackupCodeAsUsed marks a backup code as used
func (r *mfaRepository) MarkBackupCodeAsUsed(ctx context.Context, codeID string) error {
	query := `
		UPDATE mfa_backup_codes
		SET used = true, used_at = $2
		WHERE id = $1
	`

	now := time.Now()
	result, err := r.db.Pool.Exec(ctx, query, codeID, now)
	if err != nil {
		return fmt.Errorf("failed to mark backup code as used: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("backup code not found")
	}

	return nil
}

// GetUnusedBackupCodesCount returns the count of unused backup codes for a user
func (r *mfaRepository) GetUnusedBackupCodesCount(ctx context.Context, userID string) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM mfa_backup_codes
		WHERE user_id = $1 AND used = false
	`

	var count int
	err := r.db.Pool.QueryRow(ctx, query, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count backup codes: %w", err)
	}

	return count, nil
}

// DeleteAllBackupCodes deletes all backup codes for a user
func (r *mfaRepository) DeleteAllBackupCodes(ctx context.Context, userID string) error {
	query := `
		DELETE FROM mfa_backup_codes
		WHERE user_id = $1
	`

	_, err := r.db.Pool.Exec(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("failed to delete backup codes: %w", err)
	}

	return nil
}
