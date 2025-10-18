package repositories

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/pkg/database"
)

// RelationshipsRepository defines the interface for user relationship operations
type RelationshipsRepository interface {
	// Follow operations
	FollowUser(ctx context.Context, followerID, followingID string) error
	UnfollowUser(ctx context.Context, followerID, followingID string) error
	IsFollowing(ctx context.Context, followerID, followingID string) (bool, error)
	GetFollowers(ctx context.Context, userID string, limit, offset int) ([]*models.UserFollow, error)
	GetFollowing(ctx context.Context, userID string, limit, offset int) ([]*models.UserFollow, error)
	GetFollowersCount(ctx context.Context, userID string) (int, error)
	GetFollowingCount(ctx context.Context, userID string) (int, error)

	// Block operations
	BlockUser(ctx context.Context, blockerID, blockedID string) error
	UnblockUser(ctx context.Context, blockerID, blockedID string) error
	IsBlocked(ctx context.Context, blockerID, blockedID string) (bool, error)
	GetBlockedUsers(ctx context.Context, blockerID string, limit, offset int) ([]*models.UserBlock, error)

	// Relationship status
	GetRelationshipStatus(ctx context.Context, viewerID, targetUserID string) (*models.RelationshipStatus, error)
}

type relationshipsRepository struct {
	db *database.DB
}

// NewRelationshipsRepository creates a new relationships repository
func NewRelationshipsRepository(db *database.DB) RelationshipsRepository {
	return &relationshipsRepository{db: db}
}

// FollowUser creates a follow relationship
func (r *relationshipsRepository) FollowUser(ctx context.Context, followerID, followingID string) error {
	query := `
		INSERT INTO user_follows (id, follower_id, following_id, created_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (follower_id, following_id) DO NOTHING
	`

	_, err := r.db.Pool.Exec(ctx, query,
		uuid.New().String(),
		followerID,
		followingID,
		time.Now(),
	)

	return err
}

// UnfollowUser removes a follow relationship
func (r *relationshipsRepository) UnfollowUser(ctx context.Context, followerID, followingID string) error {
	query := `
		DELETE FROM user_follows
		WHERE follower_id = $1 AND following_id = $2
	`

	_, err := r.db.Pool.Exec(ctx, query, followerID, followingID)
	return err
}

// IsFollowing checks if a user is following another user
func (r *relationshipsRepository) IsFollowing(ctx context.Context, followerID, followingID string) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM user_follows
			WHERE follower_id = $1 AND following_id = $2
		)
	`

	var exists bool
	err := r.db.Pool.QueryRow(ctx, query, followerID, followingID).Scan(&exists)
	return exists, err
}

// GetFollowers gets a user's followers
func (r *relationshipsRepository) GetFollowers(ctx context.Context, userID string, limit, offset int) ([]*models.UserFollow, error) {
	query := `
		SELECT id, follower_id, following_id, created_at
		FROM user_follows
		WHERE following_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.Pool.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var follows []*models.UserFollow
	for rows.Next() {
		follow := &models.UserFollow{}
		err := rows.Scan(
			&follow.ID,
			&follow.FollowerID,
			&follow.FollowingID,
			&follow.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		follows = append(follows, follow)
	}

	return follows, rows.Err()
}

// GetFollowing gets users that a user is following
func (r *relationshipsRepository) GetFollowing(ctx context.Context, userID string, limit, offset int) ([]*models.UserFollow, error) {
	query := `
		SELECT id, follower_id, following_id, created_at
		FROM user_follows
		WHERE follower_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.Pool.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var follows []*models.UserFollow
	for rows.Next() {
		follow := &models.UserFollow{}
		err := rows.Scan(
			&follow.ID,
			&follow.FollowerID,
			&follow.FollowingID,
			&follow.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		follows = append(follows, follow)
	}

	return follows, rows.Err()
}

// GetFollowersCount gets the count of followers for a user
func (r *relationshipsRepository) GetFollowersCount(ctx context.Context, userID string) (int, error) {
	query := `
		SELECT COUNT(*) FROM user_follows
		WHERE following_id = $1
	`

	var count int
	err := r.db.Pool.QueryRow(ctx, query, userID).Scan(&count)
	return count, err
}

// GetFollowingCount gets the count of users that a user is following
func (r *relationshipsRepository) GetFollowingCount(ctx context.Context, userID string) (int, error) {
	query := `
		SELECT COUNT(*) FROM user_follows
		WHERE follower_id = $1
	`

	var count int
	err := r.db.Pool.QueryRow(ctx, query, userID).Scan(&count)
	return count, err
}

// BlockUser creates a block relationship
func (r *relationshipsRepository) BlockUser(ctx context.Context, blockerID, blockedID string) error {
	query := `
		INSERT INTO user_blocks (id, blocker_id, blocked_id, created_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (blocker_id, blocked_id) DO NOTHING
	`

	_, err := r.db.Pool.Exec(ctx, query,
		uuid.New().String(),
		blockerID,
		blockedID,
		time.Now(),
	)

	return err
}

// UnblockUser removes a block relationship
func (r *relationshipsRepository) UnblockUser(ctx context.Context, blockerID, blockedID string) error {
	query := `
		DELETE FROM user_blocks
		WHERE blocker_id = $1 AND blocked_id = $2
	`

	_, err := r.db.Pool.Exec(ctx, query, blockerID, blockedID)
	return err
}

// IsBlocked checks if a user has blocked another user
func (r *relationshipsRepository) IsBlocked(ctx context.Context, blockerID, blockedID string) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM user_blocks
			WHERE blocker_id = $1 AND blocked_id = $2
		)
	`

	var exists bool
	err := r.db.Pool.QueryRow(ctx, query, blockerID, blockedID).Scan(&exists)
	return exists, err
}

// GetBlockedUsers gets a list of blocked users
func (r *relationshipsRepository) GetBlockedUsers(ctx context.Context, blockerID string, limit, offset int) ([]*models.UserBlock, error) {
	query := `
		SELECT id, blocker_id, blocked_id, created_at
		FROM user_blocks
		WHERE blocker_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.Pool.Query(ctx, query, blockerID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var blocks []*models.UserBlock
	for rows.Next() {
		block := &models.UserBlock{}
		err := rows.Scan(
			&block.ID,
			&block.BlockerID,
			&block.BlockedID,
			&block.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		blocks = append(blocks, block)
	}

	return blocks, rows.Err()
}

// GetRelationshipStatus gets the complete relationship status between two users
func (r *relationshipsRepository) GetRelationshipStatus(ctx context.Context, viewerID, targetUserID string) (*models.RelationshipStatus, error) {
	query := `
		SELECT
			EXISTS(SELECT 1 FROM user_follows WHERE follower_id = $1 AND following_id = $2) AS is_following,
			EXISTS(SELECT 1 FROM user_follows WHERE follower_id = $2 AND following_id = $1) AS is_followed_by,
			EXISTS(SELECT 1 FROM user_blocks WHERE blocker_id = $1 AND blocked_id = $2) AS is_blocked,
			EXISTS(SELECT 1 FROM user_blocks WHERE blocker_id = $2 AND blocked_id = $1) AS has_blocked_me
	`

	status := &models.RelationshipStatus{}
	err := r.db.Pool.QueryRow(ctx, query, viewerID, targetUserID).Scan(
		&status.IsFollowing,
		&status.IsFollowedBy,
		&status.IsBlocked,
		&status.HasBlockedMe,
	)

	return status, err
}
