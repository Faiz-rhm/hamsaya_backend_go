package repositories

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/hamsaya/backend/pkg/database"
	"github.com/jackc/pgx/v5"
)

// CelebrityThreshold is the follower count above which an author is treated as a
// celebrity: their posts are NOT fanned out to user_feeds on write, but are
// queried on read instead (to avoid write-amplification for huge audiences).
const CelebrityThreshold = 10_000

// FanoutRepository handles the hybrid push/pull feed logic.
type FanoutRepository interface {
	// InsertFeedEntries pushes a post into each follower's user_feeds row.
	InsertFeedEntries(ctx context.Context, postID string, followerIDs []string) error
	// GetFollowerIDs returns all follower user IDs for the given author.
	GetFollowerIDs(ctx context.Context, authorID string) ([]string, error)
	// CountFollowers returns the follower count for the given author.
	CountFollowers(ctx context.Context, authorID string) (int, error)
	// GetPersonalizedFeed returns post IDs from user_feeds ordered by recency.
	// Returns (postIDs, nextCursor, error). nextCursor is the created_at of the
	// last returned row — pass it as cursor in the next call.
	GetPersonalizedFeed(ctx context.Context, viewerID string, cursor *time.Time, limit int) ([]string, *time.Time, error)
	// GetCelebrityPostIDs returns post IDs from followed celebrity accounts
	// (followers > CelebrityThreshold) queried directly from posts.
	GetCelebrityPostIDs(ctx context.Context, viewerID string, cursor *time.Time, limit int) ([]string, error)
}

type fanoutRepository struct{ db *database.DB }

// NewFanoutRepository creates a new FanoutRepository.
func NewFanoutRepository(db *database.DB) FanoutRepository {
	return &fanoutRepository{db: db}
}

func (r *fanoutRepository) CountFollowers(ctx context.Context, authorID string) (int, error) {
	var count int
	err := r.db.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM user_follows WHERE following_id = $1`, authorID,
	).Scan(&count)
	return count, err
}

func (r *fanoutRepository) GetFollowerIDs(ctx context.Context, authorID string) ([]string, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT follower_id FROM user_follows WHERE following_id = $1`, authorID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *fanoutRepository) InsertFeedEntries(ctx context.Context, postID string, followerIDs []string) error {
	if len(followerIDs) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	now := time.Now()
	for _, fid := range followerIDs {
		batch.Queue(
			`INSERT INTO user_feeds (id, user_id, post_id, created_at)
			 VALUES ($1, $2, $3, $4) ON CONFLICT (user_id, post_id) DO NOTHING`,
			uuid.New().String(), fid, postID, now,
		)
	}
	br := r.db.Pool.SendBatch(ctx, batch)
	defer br.Close()
	for range followerIDs {
		if _, err := br.Exec(); err != nil {
			return err
		}
	}
	return nil
}

func (r *fanoutRepository) GetPersonalizedFeed(ctx context.Context, viewerID string, cursor *time.Time, limit int) ([]string, *time.Time, error) {
	var rows pgx.Rows
	var err error
	if cursor != nil {
		rows, err = r.db.Pool.Query(ctx,
			`SELECT post_id, created_at FROM user_feeds
			 WHERE user_id = $1 AND created_at < $2
			 ORDER BY created_at DESC LIMIT $3`,
			viewerID, cursor, limit)
	} else {
		rows, err = r.db.Pool.Query(ctx,
			`SELECT post_id, created_at FROM user_feeds
			 WHERE user_id = $1
			 ORDER BY created_at DESC LIMIT $2`,
			viewerID, limit)
	}
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	var postIDs []string
	var lastCA *time.Time
	for rows.Next() {
		var pid string
		var ca time.Time
		if err := rows.Scan(&pid, &ca); err != nil {
			return nil, nil, err
		}
		postIDs = append(postIDs, pid)
		lastCA = &ca
	}
	return postIDs, lastCA, rows.Err()
}

func (r *fanoutRepository) GetCelebrityPostIDs(ctx context.Context, viewerID string, cursor *time.Time, limit int) ([]string, error) {
	cursorClause := ""
	args := []interface{}{viewerID, CelebrityThreshold, limit}
	if cursor != nil {
		cursorClause = " AND p.created_at < $4"
		args = append(args, *cursor)
	}
	query := fmt.Sprintf(`
		SELECT p.id FROM posts p
		JOIN user_follows uf ON uf.following_id = p.user_id AND uf.follower_id = $1
		JOIN (
			SELECT following_id, COUNT(*) AS fc
			FROM user_follows GROUP BY following_id
		) fc ON fc.following_id = p.user_id
		WHERE fc.fc > $2
		  AND p.deleted_at IS NULL AND p.status = true
		  %s
		ORDER BY p.created_at DESC LIMIT $3`, cursorClause)
	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
