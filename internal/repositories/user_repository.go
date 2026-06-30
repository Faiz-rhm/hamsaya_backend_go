package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
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
	// GetByOAuthProviderID retrieves a user by OAuth provider + provider user id.
	// Used to recover returning OAuth users (notably Apple) when the provider
	// omits the email claim on subsequent logins or the user hid their email.
	GetByOAuthProviderID(ctx context.Context, provider, providerUserID string) (*models.User, error)
	Update(ctx context.Context, user *models.User) error
	UpdateLoginAttempts(ctx context.Context, userID string, attempts int, lockedUntil *time.Time) error
	UpdateLastLogin(ctx context.Context, userID string) error

	// Profile operations
	CreateProfile(ctx context.Context, profile *models.Profile) error
	GetProfileByUserID(ctx context.Context, userID string) (*models.Profile, error)
	GetProfileByUserIDIncludingDeleted(ctx context.Context, userID string) (*models.Profile, error)
	GetProfilesByUserIDs(ctx context.Context, userIDs []string) ([]*models.Profile, error)
	// GetUserIDsByNeighborhood returns active profile IDs that share the given
	// province/district/neighborhood (case-insensitive), excluding excludeUserID.
	// Used to notify neighbors when someone posts in their area.
	GetUserIDsByNeighborhood(ctx context.Context, province, district, neighborhood, excludeUserID string, limit, offset int) ([]string, error)
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
	GetSessionByRefreshTokenHashAny(ctx context.Context, refreshTokenHash string) (*models.UserSession, error)
	RevokeSession(ctx context.Context, sessionID string) error
	// MarkSessionRotated revokes a session and points it at its successor so a
	// concurrent refresh-token holder can recover the cached replacement pair.
	MarkSessionRotated(ctx context.Context, sessionID, replacementSessionID string) error
	// RevokeSessionFamily revokes every active session sharing familyID. Used
	// for reuse detection — a rotated refresh token presented out of grace
	// indicates a leak, so every descendant session is killed.
	RevokeSessionFamily(ctx context.Context, familyID string) error
	RevokeAllUserSessions(ctx context.Context, userID string) error
	RevokeAllUserSessionsExcept(ctx context.Context, userID string, exceptSessionID string) error
	GetActiveSessions(ctx context.Context, userID string) ([]*models.UserSession, error)
	DeleteExpiredSessions(ctx context.Context) (int64, error)

	// Device credentials (long-lived, stored in Keychain/Keystore)
	CreateDeviceCredential(ctx context.Context, cred *models.DeviceCredential) error
	GetDeviceCredentialByHash(ctx context.Context, credentialHash string) (*models.DeviceCredential, error)
	TouchDeviceCredential(ctx context.Context, credentialID string) error
	RevokeDeviceCredential(ctx context.Context, userID, credentialID string) error
	RevokeAllUserDeviceCredentials(ctx context.Context, userID string) error
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
		INSERT INTO users (id, email, phone, phone_country_code, password_hash, email_verified, phone_verified, mfa_enabled, role,
			oauth_provider, oauth_provider_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`

	_, err := r.db.Pool.Exec(ctx, query,
		user.ID,
		user.Email,
		user.Phone,
		user.PhoneCountryCode,
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
		SELECT id, email, phone, phone_country_code, password_hash, email_verified, phone_verified, mfa_enabled, role,
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
		&user.PhoneCountryCode,
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
		SELECT id, email, phone, phone_country_code, password_hash, email_verified, phone_verified, mfa_enabled, role,
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
		&user.PhoneCountryCode,
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
		SELECT id, email, phone, phone_country_code, password_hash, email_verified, phone_verified, mfa_enabled, role,
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
		&user.PhoneCountryCode,
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

// GetByOAuthProviderID retrieves a user by OAuth provider + provider user id.
func (r *userRepository) GetByOAuthProviderID(ctx context.Context, provider, providerUserID string) (*models.User, error) {
	query := `
		SELECT id, email, phone, phone_country_code, password_hash, email_verified, phone_verified, mfa_enabled, role,
			oauth_provider, oauth_provider_id, last_login_at, failed_login_attempts,
			locked_until, created_at, updated_at, deleted_at
		FROM users
		WHERE oauth_provider = $1 AND oauth_provider_id = $2 AND deleted_at IS NULL
	`

	user := &models.User{}
	err := r.db.Pool.QueryRow(ctx, query, provider, providerUserID).Scan(
		&user.ID,
		&user.Email,
		&user.Phone,
		&user.PhoneCountryCode,
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
		return nil, fmt.Errorf("failed to get user by oauth provider id: %w", err)
	}

	return user, nil
}

// GetByEmailIncludingDeleted retrieves a user by email, including soft-deleted
func (r *userRepository) GetByEmailIncludingDeleted(ctx context.Context, email string) (*models.User, error) {
	query := `
		SELECT id, email, phone, phone_country_code, password_hash, email_verified, phone_verified, mfa_enabled, role,
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
		&user.PhoneCountryCode,
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
		SET email = $2, phone = $3, phone_country_code = $4, password_hash = $5,
			email_verified = $6, phone_verified = $7, mfa_enabled = $8, role = $9,
			updated_at = $10
		WHERE id = $1 AND deleted_at IS NULL
	`

	result, err := r.db.Pool.Exec(ctx, query,
		user.ID,
		user.Email,
		user.Phone,
		user.PhoneCountryCode,
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
	var avatarJSON []byte
	if profile.Avatar != nil {
		var err error
		avatarJSON, err = json.Marshal(profile.Avatar)
		if err != nil {
			return fmt.Errorf("failed to marshal avatar: %w", err)
		}
	}

	var query string
	var args []interface{}

	if profile.Location != nil && profile.Location.Valid {
		query = `
			INSERT INTO profiles (id, first_name, last_name, location, avatar, avatar_color, is_complete, created_at, updated_at)
			VALUES ($1, $2, $3, ST_SetSRID(ST_MakePoint($4, $5), 4326)::geography, $6, $7, $8, $9, $10)
		`
		args = []interface{}{
			profile.ID,
			profile.FirstName,
			profile.LastName,
			profile.Location.P.X,
			profile.Location.P.Y,
			avatarJSON,
			profile.AvatarColor,
			profile.IsComplete,
			profile.CreatedAt,
			profile.UpdatedAt,
		}
	} else {
		query = `
			INSERT INTO profiles (id, first_name, last_name, avatar, avatar_color, is_complete, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		`
		args = []interface{}{
			profile.ID,
			profile.FirstName,
			profile.LastName,
			avatarJSON,
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

// GetProfilesByUserIDs retrieves multiple profiles by user ID in one query.
// Soft-deleted rows are excluded. The returned slice may be shorter than the
// input when some IDs have no matching active profile.
func (r *userRepository) GetProfilesByUserIDs(ctx context.Context, userIDs []string) ([]*models.Profile, error) {
	if len(userIDs) == 0 {
		return []*models.Profile{}, nil
	}

	query := `
		SELECT id, first_name, last_name, avatar, avatar_color, cover, about, gender, dob, website,
			ST_X(location::geometry) as longitude,
			ST_Y(location::geometry) as latitude,
			country, province, district, neighborhood, is_complete,
			created_at, updated_at, deleted_at
		FROM profiles
		WHERE id = ANY($1) AND deleted_at IS NULL
	`

	rows, err := r.db.Pool.Query(ctx, query, userIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get profiles by ids: %w", err)
	}
	defer rows.Close()

	profiles := make([]*models.Profile, 0, len(userIDs))
	for rows.Next() {
		profile := &models.Profile{}
		var latitude, longitude *float64
		if err := rows.Scan(
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
		); err != nil {
			return nil, fmt.Errorf("failed to scan profile: %w", err)
		}
		if latitude != nil && longitude != nil {
			profile.Location = &pgtype.Point{
				P:     pgtype.Vec2{X: *longitude, Y: *latitude},
				Valid: true,
			}
		}
		profiles = append(profiles, profile)
	}

	return profiles, rows.Err()
}

// GetUserIDsByNeighborhood returns active profile IDs in the same
// province/district/neighborhood (case-insensitive, trimmed), excluding the
// poster. neighborhood must be non-empty; province/district narrow the match so
// identically named neighborhoods in different areas don't collide.
func (r *userRepository) GetUserIDsByNeighborhood(ctx context.Context, province, district, neighborhood, excludeUserID string, limit, offset int) ([]string, error) {
	if strings.TrimSpace(neighborhood) == "" {
		return []string{}, nil
	}

	query := `
		SELECT id
		FROM profiles
		WHERE deleted_at IS NULL
			AND id <> $1
			AND neighborhood IS NOT NULL AND TRIM(LOWER(neighborhood)) = TRIM(LOWER($2))
			AND province IS NOT NULL AND TRIM(LOWER(province)) = TRIM(LOWER($3))
			AND district IS NOT NULL AND TRIM(LOWER(district)) = TRIM(LOWER($4))
		ORDER BY id
		LIMIT $5 OFFSET $6
	`

	rows, err := r.db.Pool.Query(ctx, query, excludeUserID, neighborhood, province, district, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get user ids by neighborhood: %w", err)
	}
	defer rows.Close()

	ids := make([]string, 0, limit)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("failed to scan neighborhood user id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
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
			family_id, device_info, ip_address, user_agent, expires_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`

	// Convert device_info string to JSONB format.
	var deviceInfoJSON []byte
	if session.DeviceInfo != nil {
		raw := *session.DeviceInfo

		// Unwrap a prior round-trip. The session SELECTs read this jsonb column
		// straight into a Go string, so DeviceInfo comes back as the literal
		// text `{"device":"…"}`. On token refresh that value is re-wrapped here,
		// adding one nesting level + escaping every time — exponential growth
		// that eventually exceeds jsonb's per-string limit (SQLSTATE 54000:
		// "string too long to represent as jsonb string") and 500s the refresh.
		var wrapper map[string]string
		if json.Unmarshal([]byte(raw), &wrapper) == nil {
			if d, ok := wrapper["device"]; ok {
				raw = d
			}
		}
		// Hard cap matching the model's `max=512` validation. Guarantees the
		// value can never approach the jsonb string limit and bounds any runaway
		// re-encoding — also truncates already-bloated sessions on their next
		// refresh so affected users recover instead of staying stuck on 500.
		if len(raw) > 512 {
			raw = raw[:512]
		}

		deviceInfo := map[string]string{"device": raw}
		var err error
		deviceInfoJSON, err = json.Marshal(deviceInfo)
		if err != nil {
			return fmt.Errorf("failed to marshal device info: %w", err)
		}
	}

	// Default family to the session id itself for first-of-family rows.
	familyID := session.FamilyID
	if familyID == nil || *familyID == "" {
		familyID = &session.ID
		session.FamilyID = familyID
	}

	_, err := r.db.Pool.Exec(ctx, query,
		session.ID,
		session.UserID,
		session.RefreshToken,
		session.RefreshTokenHash,
		session.AccessTokenHash,
		familyID,
		deviceInfoJSON,
		session.IPAddress,
		session.UserAgent,
		session.ExpiresAt,
		session.CreatedAt,
		session.UpdatedAt,
	)

	return err
}

// sessionSelectCols centralises the column list so all session reads have
// matching scan order. Update this list whenever the columns change.
const sessionSelectCols = `id, user_id, refresh_token, refresh_token_hash, access_token_hash,
	family_id, replaced_by_session_id, device_info, ip_address::text, user_agent,
	expires_at, revoked, revoked_at, created_at, updated_at`

func scanSession(row interface {
	Scan(dest ...any) error
}) (*models.UserSession, error) {
	s := &models.UserSession{}
	if err := row.Scan(
		&s.ID, &s.UserID, &s.RefreshToken, &s.RefreshTokenHash, &s.AccessTokenHash,
		&s.FamilyID, &s.ReplacedBySessionID, &s.DeviceInfo, &s.IPAddress, &s.UserAgent,
		&s.ExpiresAt, &s.Revoked, &s.RevokedAt, &s.CreatedAt, &s.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return s, nil
}

// GetSessionByID retrieves a session by ID
func (r *userRepository) GetSessionByID(ctx context.Context, sessionID string) (*models.UserSession, error) {
	query := `SELECT ` + sessionSelectCols + ` FROM user_sessions WHERE id = $1`
	session, err := scanSession(r.db.Pool.QueryRow(ctx, query, sessionID))
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
	query := `SELECT ` + sessionSelectCols + ` FROM user_sessions WHERE refresh_token = $1 AND revoked = false`
	session, err := scanSession(r.db.Pool.QueryRow(ctx, query, refreshToken))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("session not found")
		}
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	return session, nil
}

// GetSessionByRefreshTokenHash retrieves the active session matching the hash.
func (r *userRepository) GetSessionByRefreshTokenHash(ctx context.Context, refreshTokenHash string) (*models.UserSession, error) {
	query := `SELECT ` + sessionSelectCols + ` FROM user_sessions WHERE refresh_token_hash = $1 AND revoked = false`
	session, err := scanSession(r.db.Pool.QueryRow(ctx, query, refreshTokenHash))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("session not found")
		}
		return nil, fmt.Errorf("failed to get session: %w", err)
	}
	return session, nil
}

// GetSessionByRefreshTokenHashAny finds the session matching the hash even if
// it has been revoked. Used by /auth/refresh to detect rotated-but-in-grace
// tokens and to drive reuse-detection on out-of-grace replays.
func (r *userRepository) GetSessionByRefreshTokenHashAny(ctx context.Context, refreshTokenHash string) (*models.UserSession, error) {
	query := `SELECT ` + sessionSelectCols + ` FROM user_sessions WHERE refresh_token_hash = $1
		ORDER BY created_at DESC LIMIT 1`
	session, err := scanSession(r.db.Pool.QueryRow(ctx, query, refreshTokenHash))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("session not found")
		}
		return nil, fmt.Errorf("failed to get session: %w", err)
	}
	return session, nil
}

// MarkSessionRotated revokes the old session and points it at its replacement
// in a single statement so concurrent reads always see consistent state.
func (r *userRepository) MarkSessionRotated(ctx context.Context, sessionID, replacementSessionID string) error {
	query := `
		UPDATE user_sessions
		SET revoked = true, revoked_at = $2, replaced_by_session_id = $3, updated_at = $2
		WHERE id = $1
	`
	now := time.Now()
	_, err := r.db.Pool.Exec(ctx, query, sessionID, now, replacementSessionID)
	return err
}

// RevokeSessionFamily revokes every active session sharing the family. Called
// when reuse detection trips so a leaked refresh token can't keep minting
// fresh access tokens through descendant sessions.
func (r *userRepository) RevokeSessionFamily(ctx context.Context, familyID string) error {
	query := `
		UPDATE user_sessions
		SET revoked = true, revoked_at = $2, updated_at = $2
		WHERE family_id = $1 AND revoked = false
	`
	_, err := r.db.Pool.Exec(ctx, query, familyID, time.Now())
	return err
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
	query := `SELECT ` + sessionSelectCols + ` FROM user_sessions
		WHERE user_id = $1 AND revoked = false AND expires_at > NOW()
		ORDER BY created_at DESC`

	rows, err := r.db.Pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get sessions: %w", err)
	}
	defer rows.Close()

	var sessions []*models.UserSession
	for rows.Next() {
		session, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, session)
	}

	return sessions, rows.Err()
}

// CreateDeviceCredential inserts a new device credential row. The hash is
// recomputed from the plaintext on every login attempt; plaintext is never
// persisted server-side.
func (r *userRepository) CreateDeviceCredential(ctx context.Context, cred *models.DeviceCredential) error {
	query := `
		INSERT INTO device_credentials (id, user_id, credential_hash, install_id, device_name,
			platform, expires_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err := r.db.Pool.Exec(ctx, query,
		cred.ID, cred.UserID, cred.CredentialHash, cred.InstallID, cred.DeviceName,
		cred.Platform, cred.ExpiresAt, cred.CreatedAt, cred.UpdatedAt,
	)
	return err
}

// GetDeviceCredentialByHash returns the active credential matching the hash.
func (r *userRepository) GetDeviceCredentialByHash(ctx context.Context, credentialHash string) (*models.DeviceCredential, error) {
	query := `
		SELECT id, user_id, credential_hash, install_id, device_name, platform,
			expires_at, revoked, revoked_at, last_used_at, created_at, updated_at
		FROM device_credentials
		WHERE credential_hash = $1 AND revoked = false
	`
	c := &models.DeviceCredential{}
	err := r.db.Pool.QueryRow(ctx, query, credentialHash).Scan(
		&c.ID, &c.UserID, &c.CredentialHash, &c.InstallID, &c.DeviceName, &c.Platform,
		&c.ExpiresAt, &c.Revoked, &c.RevokedAt, &c.LastUsedAt, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("device credential not found")
		}
		return nil, fmt.Errorf("failed to get device credential: %w", err)
	}
	return c, nil
}

// TouchDeviceCredential bumps last_used_at for usage telemetry. Failures are
// non-fatal — callers may log and continue.
func (r *userRepository) TouchDeviceCredential(ctx context.Context, credentialID string) error {
	_, err := r.db.Pool.Exec(ctx, `UPDATE device_credentials SET last_used_at = $2, updated_at = $2 WHERE id = $1`,
		credentialID, time.Now())
	return err
}

// ErrDeviceCredentialNotFound is returned when a revoke targets a credential
// that doesn't exist or isn't owned by the caller.
var ErrDeviceCredentialNotFound = errors.New("device credential not found")

// RevokeDeviceCredential marks a single credential dead. Existing sessions
// minted from it remain valid until their natural expiry.
//
// Scoped by user_id: a caller can only revoke a credential they own. Returns
// ErrDeviceCredentialNotFound when no row matches (wrong owner or unknown id)
// so the service layer can answer 404 instead of silently succeeding — this
// is the guard against the IDOR where any user could revoke any credential.
func (r *userRepository) RevokeDeviceCredential(ctx context.Context, userID, credentialID string) error {
	tag, err := r.db.Pool.Exec(ctx,
		`UPDATE device_credentials SET revoked = true, revoked_at = $3, updated_at = $3 WHERE id = $1 AND user_id = $2`,
		credentialID, userID, time.Now())
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrDeviceCredentialNotFound
	}
	return nil
}

// RevokeAllUserDeviceCredentials revokes every device credential for a user.
// Used on password reset, account compromise reports, and explicit "log out
// of every device" actions.
func (r *userRepository) RevokeAllUserDeviceCredentials(ctx context.Context, userID string) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE device_credentials SET revoked = true, revoked_at = $2, updated_at = $2 WHERE user_id = $1 AND revoked = false`,
		userID, time.Now())
	return err
}

// CreateUserWithProfile creates a user and their profile atomically within a transaction.
// If either operation fails, both are rolled back.
func (r *userRepository) CreateUserWithProfile(ctx context.Context, user *models.User, profile *models.Profile) error {
	return r.db.WithTransaction(ctx, func(tx pgx.Tx) error {
		// Create user
		userQuery := `
			INSERT INTO users (id, email, phone, phone_country_code, password_hash, email_verified, phone_verified, mfa_enabled, role,
				oauth_provider, oauth_provider_id, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		`
		_, err := tx.Exec(ctx, userQuery,
			user.ID, user.Email, user.Phone, user.PhoneCountryCode, user.PasswordHash,
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

// DeleteExpiredSessions removes sessions that are either revoked or past their expiry date.
func (r *userRepository) DeleteExpiredSessions(ctx context.Context) (int64, error) {
	tag, err := r.db.Pool.Exec(ctx, `
		DELETE FROM user_sessions
		WHERE revoked = true OR expires_at < NOW()
	`)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
