package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/pkg/database"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

// UserRepository defines the interface for user data operations
type UserRepository interface {
	// User operations
	Create(ctx context.Context, user *models.User) error
	GetByID(ctx context.Context, id string) (*models.User, error)
	GetByIDIncludingDeleted(ctx context.Context, id string) (*models.User, error)
	GetByEmail(ctx context.Context, email string) (*models.User, error)
	GetByEmailIncludingDeleted(ctx context.Context, email string) (*models.User, error)
	Update(ctx context.Context, user *models.User) error
	UpdateLoginAttempts(ctx context.Context, userID string, attempts int, lockedUntil *time.Time) error
	UpdateLastLogin(ctx context.Context, userID string) error

	// Profile operations
	CreateProfile(ctx context.Context, profile *models.Profile) error
	GetProfileByUserID(ctx context.Context, userID string) (*models.Profile, error)
	GetProfileByUserIDIncludingDeleted(ctx context.Context, userID string) (*models.Profile, error)
	UpdateProfile(ctx context.Context, profile *models.Profile) error

	// Transactional operations
	CreateUserWithProfile(ctx context.Context, user *models.User, profile *models.Profile) error

	// Soft delete (deactivate) user
	SoftDelete(ctx context.Context, userID string) error
	// Restore reactivates a soft-deleted user
	Restore(ctx context.Context, userID string) error

	// Session operations
	CreateSession(ctx context.Context, session *models.UserSession) error
	GetSessionByID(ctx context.Context, sessionID string) (*models.UserSession, error)
	GetSessionByRefreshToken(ctx context.Context, refreshToken string) (*models.UserSession, error)
	GetSessionByRefreshTokenHash(ctx context.Context, refreshTokenHash string) (*models.UserSession, error)
	RevokeSession(ctx context.Context, sessionID string) error
	RevokeAllUserSessions(ctx context.Context, userID string) error
	RevokeAllUserSessionsExcept(ctx context.Context, userID string, exceptSessionID string) error
	GetActiveSessions(ctx context.Context, userID string) ([]*models.UserSession, error)
}

type userRepository struct {
	db *database.DB
}

// NewUserRepository creates a new user repository
func NewUserRepository(db *database.DB) UserRepository {
	return &userRepository{db: db}
}

// Create creates a new user
func (r *userRepository) Create(ctx context.Context, user *models.User) error {
	query := `
		INSERT INTO users (id, email, phone, password_hash, email_verified, phone_verified, mfa_enabled, role,
			oauth_provider, oauth_provider_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`

	_, err := r.db.Pool.Exec(ctx, query,
		user.ID,
		user.Email,
		user.Phone,
		user.PasswordHash,
		user.EmailVerified,
		user.PhoneVerified,
		user.MFAEnabled,
		user.Role,
		user.OAuthProvider,
		user.OAuthProviderID,
		user.CreatedAt,
		user.UpdatedAt,
	)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
			return fmt.Errorf("user with email already exists")
		}
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}

// GetByID retrieves a user by ID
func (r *userRepository) GetByID(ctx context.Context, id string) (*models.User, error) {
	query := `
		SELECT id, email, phone, password_hash, email_verified, phone_verified, mfa_enabled, role,
			oauth_provider, oauth_provider_id, last_login_at, failed_login_attempts,
			locked_until, created_at, updated_at, deleted_at
		FROM users
		WHERE id = $1 AND deleted_at IS NULL
	`

	user := &models.User{}
	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&user.ID,
		&user.Email,
		&user.Phone,
		&user.PasswordHash,
		&user.EmailVerified,
		&user.PhoneVerified,
		&user.MFAEnabled,
		&user.Role,
		&user.OAuthProvider,
		&user.OAuthProviderID,
		&user.LastLoginAt,
		&user.FailedLoginAttempts,
		&user.LockedUntil,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.DeletedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return user, nil
}

// GetByIDIncludingDeleted retrieves a user by ID even if soft-deleted
func (r *userRepository) GetByIDIncludingDeleted(ctx context.Context, id string) (*models.User, error) {
	query := `
		SELECT id, email, phone, password_hash, email_verified, phone_verified, mfa_enabled, role,
			oauth_provider, oauth_provider_id, last_login_at, failed_login_attempts,
			locked_until, created_at, updated_at, deleted_at
		FROM users
		WHERE id = $1
	`

	user := &models.User{}
	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&user.ID,
		&user.Email,
		&user.Phone,
		&user.PasswordHash,
		&user.EmailVerified,
		&user.PhoneVerified,
		&user.MFAEnabled,
		&user.Role,
		&user.OAuthProvider,
		&user.OAuthProviderID,
		&user.LastLoginAt,
		&user.FailedLoginAttempts,
		&user.LockedUntil,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.DeletedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return user, nil
}

// GetByEmail retrieves a user by email
func (r *userRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	query := `
		SELECT id, email, phone, password_hash, email_verified, phone_verified, mfa_enabled, role,
			oauth_provider, oauth_provider_id, last_login_at, failed_login_attempts,
			locked_until, created_at, updated_at, deleted_at
		FROM users
		WHERE email = $1 AND deleted_at IS NULL
	`

	user := &models.User{}
	err := r.db.Pool.QueryRow(ctx, query, email).Scan(
		&user.ID,
		&user.Email,
		&user.Phone,
		&user.PasswordHash,
		&user.EmailVerified,
		&user.PhoneVerified,
		&user.MFAEnabled,
		&user.Role,
		&user.OAuthProvider,
		&user.OAuthProviderID,
		&user.LastLoginAt,
		&user.FailedLoginAttempts,
		&user.LockedUntil,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.DeletedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return user, nil
}

// GetByEmailIncludingDeleted retrieves a user by email, including soft-deleted
func (r *userRepository) GetByEmailIncludingDeleted(ctx context.Context, email string) (*models.User, error) {
	query := `
		SELECT id, email, phone, password_hash, email_verified, phone_verified, mfa_enabled, role,
			oauth_provider, oauth_provider_id, last_login_at, failed_login_attempts,
			locked_until, created_at, updated_at, deleted_at
		FROM users
		WHERE email = $1
	`

	user := &models.User{}
	err := r.db.Pool.QueryRow(ctx, query, email).Scan(
		&user.ID,
		&user.Email,
		&user.Phone,
		&user.PasswordHash,
		&user.EmailVerified,
		&user.PhoneVerified,
		&user.MFAEnabled,
		&user.Role,
		&user.OAuthProvider,
		&user.OAuthProviderID,
		&user.LastLoginAt,
		&user.FailedLoginAttempts,
		&user.LockedUntil,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.DeletedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return user, nil
}

// Update updates a user
func (r *userRepository) Update(ctx context.Context, user *models.User) error {
	query := `
		UPDATE users
		SET email = $2, phone = $3, password_hash = $4, email_verified = $5,
			phone_verified = $6, mfa_enabled = $7, role = $8, updated_at = $9
		WHERE id = $1 AND deleted_at IS NULL
	`

	result, err := r.db.Pool.Exec(ctx, query,
		user.ID,
		user.Email,
		user.Phone,
		user.PasswordHash,
		user.EmailVerified,
		user.PhoneVerified,
		user.MFAEnabled,
		user.Role,
		time.Now(),
	)

	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("user not found")
	}

	return nil
}

// UpdateLoginAttempts updates failed login attempts and lock status
func (r *userRepository) UpdateLoginAttempts(ctx context.Context, userID string, attempts int, lockedUntil *time.Time) error {
	query := `
		UPDATE users
		SET failed_login_attempts = $2, locked_until = $3, updated_at = $4
		WHERE id = $1
	`

	_, err := r.db.Pool.Exec(ctx, query, userID, attempts, lockedUntil, time.Now())
	return err
}

// UpdateLastLogin updates the last login timestamp
func (r *userRepository) UpdateLastLogin(ctx context.Context, userID string) error {
	query := `
		UPDATE users
		SET last_login_at = $2, failed_login_attempts = 0, locked_until = NULL, updated_at = $3
		WHERE id = $1
	`

	_, err := r.db.Pool.Exec(ctx, query, userID, time.Now(), time.Now())
	return err
}

// CreateProfile creates a new user profile
func (r *userRepository) CreateProfile(ctx context.Context, profile *models.Profile) error {
	var query string
	var args []interface{}

	if profile.Location != nil && profile.Location.Valid {
		// Use PostGIS function to create GEOGRAPHY point
		query = `
			INSERT INTO profiles (id, first_name, last_name, location, avatar_color, is_complete, created_at, updated_at)
			VALUES ($1, $2, $3, ST_SetSRID(ST_MakePoint($4, $5), 4326)::geography, $6, $7, $8, $9)
		`
		args = []interface{}{
			profile.ID,
			profile.FirstName,
			profile.LastName,
			profile.Location.P.X, // Longitude
			profile.Location.P.Y, // Latitude
			profile.AvatarColor,
			profile.IsComplete,
			profile.CreatedAt,
			profile.UpdatedAt,
		}
	} else {
		query = `
			INSERT INTO profiles (id, first_name, last_name, avatar_color, is_complete, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`
		args = []interface{}{
			profile.ID,
			profile.FirstName,
			profile.LastName,
			profile.AvatarColor,
			profile.IsComplete,
			profile.CreatedAt,
			profile.UpdatedAt,
		}
	}

	_, err := r.db.Pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to create profile: %w", err)
	}

	return nil
}

// GetProfileByUserID retrieves a profile by user ID
func (r *userRepository) GetProfileByUserID(ctx context.Context, userID string) (*models.Profile, error) {
	query := `
		SELECT id, first_name, last_name, avatar, avatar_color, cover, about, gender, dob, website,
			ST_X(location::geometry) as longitude,
			ST_Y(location::geometry) as latitude,
			country, province, district, neighborhood, is_complete,
			created_at, updated_at, deleted_at
		FROM profiles
		WHERE id = $1 AND deleted_at IS NULL
	`

	profile := &models.Profile{}
	var latitude, longitude *float64
	err := r.db.Pool.QueryRow(ctx, query, userID).Scan(
		&profile.ID,
		&profile.FirstName,
		&profile.LastName,
		&profile.Avatar,
		&profile.AvatarColor,
		&profile.Cover,
		&profile.About,
		&profile.Gender,
		&profile.DOB,
		&profile.Website,
		&longitude,
		&latitude,
		&profile.Country,
		&profile.Province,
		&profile.District,
		&profile.Neighborhood,
		&profile.IsComplete,
		&profile.CreatedAt,
		&profile.UpdatedAt,
		&profile.DeletedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("profile not found")
		}
		return nil, fmt.Errorf("failed to get profile: %w", err)
	}

	// Construct pgtype.Point from latitude and longitude if both exist
	if latitude != nil && longitude != nil {
		profile.Location = &pgtype.Point{
			P:     pgtype.Vec2{X: *longitude, Y: *latitude},
			Valid: true,
		}
	}

	return profile, nil
}

// GetProfileByUserIDIncludingDeleted retrieves a profile by user ID, including soft-deleted
func (r *userRepository) GetProfileByUserIDIncludingDeleted(ctx context.Context, userID string) (*models.Profile, error) {
	query := `
		SELECT id, first_name, last_name, avatar, avatar_color, cover, about, gender, dob, website,
			ST_X(location::geometry) as longitude,
			ST_Y(location::geometry) as latitude,
			country, province, district, neighborhood, is_complete,
			created_at, updated_at, deleted_at
		FROM profiles
		WHERE id = $1
	`

	profile := &models.Profile{}
	var latitude, longitude *float64
	err := r.db.Pool.QueryRow(ctx, query, userID).Scan(
		&profile.ID,
		&profile.FirstName,
		&profile.LastName,
		&profile.Avatar,
		&profile.AvatarColor,
		&profile.Cover,
		&profile.About,
		&profile.Gender,
		&profile.DOB,
		&profile.Website,
		&longitude,
		&latitude,
		&profile.Country,
		&profile.Province,
		&profile.District,
		&profile.Neighborhood,
		&profile.IsComplete,
		&profile.CreatedAt,
		&profile.UpdatedAt,
		&profile.DeletedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("profile not found")
		}
		return nil, fmt.Errorf("failed to get profile: %w", err)
	}

	if latitude != nil && longitude != nil {
		profile.Location = &pgtype.Point{
			P:     pgtype.Vec2{X: *longitude, Y: *latitude},
			Valid: true,
		}
	}

	return profile, nil
}

// UpdateProfile updates a user profile
func (r *userRepository) UpdateProfile(ctx context.Context, profile *models.Profile) error {
	// Build query based on whether location is provided
	var query string
	var args []interface{}

	if profile.Location != nil && profile.Location.Valid {
		// Use PostGIS function to create GEOGRAPHY point from longitude and latitude
		query = `
			UPDATE profiles
			SET first_name = $2, last_name = $3,
				location = ST_SetSRID(ST_MakePoint($4, $5), 4326)::geography,
				about = $6, gender = $7, dob = $8, website = $9, country = $10,
				province = $11, district = $12, neighborhood = $13, avatar = $14, avatar_color = $15, cover = $16,
				is_complete = $17, updated_at = $18
			WHERE id = $1 AND deleted_at IS NULL
		`
		args = []interface{}{
			profile.ID,
			profile.FirstName,
			profile.LastName,
			profile.Location.P.X, // Longitude
			profile.Location.P.Y, // Latitude
			profile.About,
			profile.Gender,
			profile.DOB,
			profile.Website,
			profile.Country,
			profile.Province,
			profile.District,
			profile.Neighborhood,
			profile.Avatar,
			profile.AvatarColor,
			profile.Cover,
			profile.IsComplete,
			time.Now(),
		}
	} else {
		query = `
			UPDATE profiles
			SET first_name = $2, last_name = $3, about = $4, gender = $5,
				dob = $6, website = $7, country = $8, province = $9,
				district = $10, neighborhood = $11, avatar = $12, avatar_color = $13, cover = $14,
				is_complete = $15, updated_at = $16
			WHERE id = $1 AND deleted_at IS NULL
		`
		args = []interface{}{
			profile.ID,
			profile.FirstName,
			profile.LastName,
			profile.About,
			profile.Gender,
			profile.DOB,
			profile.Website,
			profile.Country,
			profile.Province,
			profile.District,
			profile.Neighborhood,
			profile.Avatar,
			profile.AvatarColor,
			profile.Cover,
			profile.IsComplete,
			time.Now(),
		}
	}

	result, err := r.db.Pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to update profile: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("profile not found")
	}

	return nil
}

// CreateSession creates a new user session
func (r *userRepository) CreateSession(ctx context.Context, session *models.UserSession) error {
	query := `
		INSERT INTO user_sessions (id, user_id, refresh_token, refresh_token_hash, access_token_hash,
			device_info, ip_address, user_agent, expires_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`

	// Convert device_info string to JSONB format
	var deviceInfoJSON []byte
	if session.DeviceInfo != nil {
		// Wrap the string in a JSON object
		deviceInfo := map[string]string{"device": *session.DeviceInfo}
		var err error
		deviceInfoJSON, err = json.Marshal(deviceInfo)
		if err != nil {
			return fmt.Errorf("failed to marshal device info: %w", err)
		}
	}

	_, err := r.db.Pool.Exec(ctx, query,
		session.ID,
		session.UserID,
		session.RefreshToken,
		session.RefreshTokenHash,
		session.AccessTokenHash,
		deviceInfoJSON,
		session.IPAddress,
		session.UserAgent,
		session.ExpiresAt,
		session.CreatedAt,
		session.UpdatedAt,
	)

	return err
}

// GetSessionByID retrieves a session by ID
func (r *userRepository) GetSessionByID(ctx context.Context, sessionID string) (*models.UserSession, error) {
	query := `
		SELECT id, user_id, refresh_token, access_token_hash, device_info,
			ip_address::text, user_agent, expires_at, revoked, revoked_at, created_at, updated_at
		FROM user_sessions
		WHERE id = $1
	`

	session := &models.UserSession{}
	err := r.db.Pool.QueryRow(ctx, query, sessionID).Scan(
		&session.ID,
		&session.UserID,
		&session.RefreshToken,
		&session.AccessTokenHash,
		&session.DeviceInfo,
		&session.IPAddress,
		&session.UserAgent,
		&session.ExpiresAt,
		&session.Revoked,
		&session.RevokedAt,
		&session.CreatedAt,
		&session.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("session not found")
		}
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	return session, nil
}

// GetSessionByRefreshToken retrieves a session by refresh token
func (r *userRepository) GetSessionByRefreshToken(ctx context.Context, refreshToken string) (*models.UserSession, error) {
	query := `
		SELECT id, user_id, refresh_token, access_token_hash, device_info,
			ip_address::text, user_agent, expires_at, revoked, revoked_at, created_at, updated_at
		FROM user_sessions
		WHERE refresh_token = $1 AND revoked = false
	`

	session := &models.UserSession{}
	err := r.db.Pool.QueryRow(ctx, query, refreshToken).Scan(
		&session.ID,
		&session.UserID,
		&session.RefreshToken,
		&session.AccessTokenHash,
		&session.DeviceInfo,
		&session.IPAddress,
		&session.UserAgent,
		&session.ExpiresAt,
		&session.Revoked,
		&session.RevokedAt,
		&session.CreatedAt,
		&session.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("session not found")
		}
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	return session, nil
}

// GetSessionByRefreshTokenHash retrieves a session by the hashed refresh token.
// This is the secure lookup method used after the hashing migration.
func (r *userRepository) GetSessionByRefreshTokenHash(ctx context.Context, refreshTokenHash string) (*models.UserSession, error) {
	query := `
		SELECT id, user_id, refresh_token, access_token_hash, device_info,
			ip_address::text, user_agent, expires_at, revoked, revoked_at, created_at, updated_at
		FROM user_sessions
		WHERE refresh_token_hash = $1 AND revoked = false
	`

	session := &models.UserSession{}
	err := r.db.Pool.QueryRow(ctx, query, refreshTokenHash).Scan(
		&session.ID,
		&session.UserID,
		&session.RefreshToken,
		&session.AccessTokenHash,
		&session.DeviceInfo,
		&session.IPAddress,
		&session.UserAgent,
		&session.ExpiresAt,
		&session.Revoked,
		&session.RevokedAt,
		&session.CreatedAt,
		&session.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("session not found")
		}
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	return session, nil
}

// SoftDelete soft-deletes a user by setting deleted_at (deactivate account)
func (r *userRepository) SoftDelete(ctx context.Context, userID string) error {
	query := `
		UPDATE users
		SET deleted_at = $2, updated_at = $2
		WHERE id = $1 AND deleted_at IS NULL
	`
	result, err := r.db.Pool.Exec(ctx, query, userID, time.Now())
	if err != nil {
		return fmt.Errorf("failed to soft delete user: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("user not found or already deleted")
	}
	return nil
}

// Restore reactivates a soft-deleted user by clearing deleted_at
func (r *userRepository) Restore(ctx context.Context, userID string) error {
	query := `
		UPDATE users
		SET deleted_at = NULL, updated_at = $2
		WHERE id = $1 AND deleted_at IS NOT NULL
	`
	result, err := r.db.Pool.Exec(ctx, query, userID, time.Now())
	if err != nil {
		return fmt.Errorf("failed to restore user: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("user not found or not deleted")
	}
	return nil
}

// RevokeSession revokes a specific session
func (r *userRepository) RevokeSession(ctx context.Context, sessionID string) error {
	query := `
		UPDATE user_sessions
		SET revoked = true, revoked_at = $2, updated_at = $3
		WHERE id = $1
	`

	_, err := r.db.Pool.Exec(ctx, query, sessionID, time.Now(), time.Now())
	return err
}

// RevokeAllUserSessions revokes all sessions for a user
func (r *userRepository) RevokeAllUserSessions(ctx context.Context, userID string) error {
	query := `
		UPDATE user_sessions
		SET revoked = true, revoked_at = $2, updated_at = $3
		WHERE user_id = $1 AND revoked = false
	`

	_, err := r.db.Pool.Exec(ctx, query, userID, time.Now(), time.Now())
	return err
}

// GetActiveSessions retrieves all active sessions for a user
func (r *userRepository) GetActiveSessions(ctx context.Context, userID string) ([]*models.UserSession, error) {
	query := `
		SELECT id, user_id, refresh_token, access_token_hash, device_info,
			ip_address::text, user_agent, expires_at, revoked, revoked_at, created_at, updated_at
		FROM user_sessions
		WHERE user_id = $1 AND revoked = false AND expires_at > NOW()
		ORDER BY created_at DESC
	`

	rows, err := r.db.Pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get sessions: %w", err)
	}
	defer rows.Close()

	var sessions []*models.UserSession
	for rows.Next() {
		session := &models.UserSession{}
		err := rows.Scan(
			&session.ID,
			&session.UserID,
			&session.RefreshToken,
			&session.AccessTokenHash,
			&session.DeviceInfo,
			&session.IPAddress,
			&session.UserAgent,
			&session.ExpiresAt,
			&session.Revoked,
			&session.RevokedAt,
			&session.CreatedAt,
			&session.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, session)
	}

	return sessions, rows.Err()
}

// CreateUserWithProfile creates a user and their profile atomically within a transaction.
// If either operation fails, both are rolled back.
func (r *userRepository) CreateUserWithProfile(ctx context.Context, user *models.User, profile *models.Profile) error {
	return r.db.WithTransaction(ctx, func(tx pgx.Tx) error {
		// Create user
		userQuery := `
			INSERT INTO users (id, email, phone, password_hash, email_verified, phone_verified, mfa_enabled, role,
				oauth_provider, oauth_provider_id, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		`
		_, err := tx.Exec(ctx, userQuery,
			user.ID, user.Email, user.Phone, user.PasswordHash,
			user.EmailVerified, user.PhoneVerified, user.MFAEnabled, user.Role,
			user.OAuthProvider, user.OAuthProviderID, user.CreatedAt, user.UpdatedAt,
		)
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				return fmt.Errorf("user with email already exists")
			}
			return fmt.Errorf("failed to create user: %w", err)
		}

		// Create profile
		if profile.Location != nil && profile.Location.Valid {
			profileQuery := `
				INSERT INTO profiles (id, first_name, last_name, location, avatar_color, is_complete, created_at, updated_at)
				VALUES ($1, $2, $3, ST_SetSRID(ST_MakePoint($4, $5), 4326)::geography, $6, $7, $8, $9)
			`
			_, err = tx.Exec(ctx, profileQuery,
				profile.ID, profile.FirstName, profile.LastName,
				profile.Location.P.X, profile.Location.P.Y,
				profile.AvatarColor,
				profile.IsComplete, profile.CreatedAt, profile.UpdatedAt,
			)
		} else {
			profileQuery := `
				INSERT INTO profiles (id, first_name, last_name, avatar_color, is_complete, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7)
			`
			_, err = tx.Exec(ctx, profileQuery,
				profile.ID, profile.FirstName, profile.LastName,
				profile.AvatarColor,
				profile.IsComplete, profile.CreatedAt, profile.UpdatedAt,
			)
		}
		if err != nil {
			return fmt.Errorf("failed to create profile: %w", err)
		}

		return nil
	})
}

// RevokeAllUserSessionsExcept revokes all sessions for a user except the specified session.
func (r *userRepository) RevokeAllUserSessionsExcept(ctx context.Context, userID string, exceptSessionID string) error {
	query := `
		UPDATE user_sessions
		SET revoked = true, revoked_at = $3, updated_at = $4
		WHERE user_id = $1 AND id != $2 AND revoked = false
	`

	now := time.Now()
	_, err := r.db.Pool.Exec(ctx, query, userID, exceptSessionID, now, now)
	return err
}
