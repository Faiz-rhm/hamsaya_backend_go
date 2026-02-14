package repositories

import (
	"context"
	"fmt"
	"time"

	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/pkg/database"
	"github.com/jackc/pgx/v5"
)

// PollRepository defines the interface for poll operations
type PollRepository interface {
	// Poll CRUD
	Create(ctx context.Context, poll *models.Poll) error
	GetByID(ctx context.Context, pollID string) (*models.Poll, error)
	GetByPostID(ctx context.Context, postID string) (*models.Poll, error)
	Delete(ctx context.Context, pollID string) error

	// Poll Options
	CreateOption(ctx context.Context, option *models.PollOption) error
	GetOptionsByPollID(ctx context.Context, pollID string) ([]*models.PollOption, error)
	GetOptionByID(ctx context.Context, optionID string) (*models.PollOption, error)
	UpdateOptionVoteCount(ctx context.Context, optionID string, increment int) error
	DeleteOptionsByPollID(ctx context.Context, pollID string) error

	// User Votes
	VotePoll(ctx context.Context, vote *models.UserPoll) error
	ChangeVote(ctx context.Context, userID, pollID, newOptionID string) error
	DeleteVote(ctx context.Context, userID, pollID string) error
	GetUserVote(ctx context.Context, userID, pollID string) (*models.UserPoll, error)
	HasUserVoted(ctx context.Context, userID, pollID string) (bool, error)
}

type pollRepository struct {
	db *database.DB
}

// NewPollRepository creates a new poll repository
func NewPollRepository(db *database.DB) PollRepository {
	return &pollRepository{db: db}
}

// Create creates a new poll
func (r *pollRepository) Create(ctx context.Context, poll *models.Poll) error {
	query := `
		INSERT INTO polls (id, post_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4)
	`

	_, err := r.db.Pool.Exec(ctx, query,
		poll.ID,
		poll.PostID,
		poll.CreatedAt,
		poll.UpdatedAt,
	)

	return err
}

// GetByID gets a poll by ID
func (r *pollRepository) GetByID(ctx context.Context, pollID string) (*models.Poll, error) {
	query := `
		SELECT id, post_id, created_at, updated_at, deleted_at
		FROM polls
		WHERE id = $1 AND deleted_at IS NULL
	`

	poll := &models.Poll{}
	err := r.db.Pool.QueryRow(ctx, query, pollID).Scan(
		&poll.ID,
		&poll.PostID,
		&poll.CreatedAt,
		&poll.UpdatedAt,
		&poll.DeletedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("poll not found")
	}

	return poll, err
}

// GetByPostID gets a poll by post ID
func (r *pollRepository) GetByPostID(ctx context.Context, postID string) (*models.Poll, error) {
	query := `
		SELECT id, post_id, created_at, updated_at, deleted_at
		FROM polls
		WHERE post_id = $1 AND deleted_at IS NULL
	`

	poll := &models.Poll{}
	err := r.db.Pool.QueryRow(ctx, query, postID).Scan(
		&poll.ID,
		&poll.PostID,
		&poll.CreatedAt,
		&poll.UpdatedAt,
		&poll.DeletedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("poll not found")
	}

	return poll, err
}

// Delete soft deletes a poll
func (r *pollRepository) Delete(ctx context.Context, pollID string) error {
	query := `
		UPDATE polls
		SET deleted_at = $2
		WHERE id = $1 AND deleted_at IS NULL
	`

	_, err := r.db.Pool.Exec(ctx, query, pollID, time.Now())
	return err
}

// CreateOption creates a poll option
func (r *pollRepository) CreateOption(ctx context.Context, option *models.PollOption) error {
	query := `
		INSERT INTO poll_options (id, poll_id, option, vote_count, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`

	_, err := r.db.Pool.Exec(ctx, query,
		option.ID,
		option.PollID,
		option.Option,
		option.VoteCount,
		option.CreatedAt,
		option.UpdatedAt,
	)

	return err
}

// GetOptionsByPollID gets all options for a poll
func (r *pollRepository) GetOptionsByPollID(ctx context.Context, pollID string) ([]*models.PollOption, error) {
	query := `
		SELECT id, poll_id, option, vote_count, created_at, updated_at
		FROM poll_options
		WHERE poll_id = $1 AND deleted_at IS NULL
		ORDER BY created_at ASC
	`

	rows, err := r.db.Pool.Query(ctx, query, pollID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var options []*models.PollOption
	for rows.Next() {
		option := &models.PollOption{}
		err := rows.Scan(
			&option.ID,
			&option.PollID,
			&option.Option,
			&option.VoteCount,
			&option.CreatedAt,
			&option.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		options = append(options, option)
	}

	return options, rows.Err()
}

// GetOptionByID gets a poll option by ID
func (r *pollRepository) GetOptionByID(ctx context.Context, optionID string) (*models.PollOption, error) {
	query := `
		SELECT id, poll_id, option, vote_count, created_at, updated_at
		FROM poll_options
		WHERE id = $1 AND deleted_at IS NULL
	`

	option := &models.PollOption{}
	err := r.db.Pool.QueryRow(ctx, query, optionID).Scan(
		&option.ID,
		&option.PollID,
		&option.Option,
		&option.VoteCount,
		&option.CreatedAt,
		&option.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("poll option not found")
	}

	return option, err
}

// UpdateOptionVoteCount updates the vote count for an option
func (r *pollRepository) UpdateOptionVoteCount(ctx context.Context, optionID string, increment int) error {
	query := `
		UPDATE poll_options
		SET vote_count = vote_count + $2, updated_at = $3
		WHERE id = $1 AND deleted_at IS NULL
	`

	_, err := r.db.Pool.Exec(ctx, query, optionID, increment, time.Now())
	return err
}

// DeleteOptionsByPollID soft-deletes all options for a poll (e.g. before replacing on update).
func (r *pollRepository) DeleteOptionsByPollID(ctx context.Context, pollID string) error {
	query := `
		UPDATE poll_options
		SET deleted_at = $2
		WHERE poll_id = $1 AND deleted_at IS NULL
	`
	_, err := r.db.Pool.Exec(ctx, query, pollID, time.Now())
	return err
}

// VotePoll creates a vote record
func (r *pollRepository) VotePoll(ctx context.Context, vote *models.UserPoll) error {
	query := `
		INSERT INTO user_polls (id, user_id, poll_id, poll_option_id, created_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id, poll_id) DO UPDATE
		SET poll_option_id = EXCLUDED.poll_option_id,
		    deleted_at = NULL
	`

	_, err := r.db.Pool.Exec(ctx, query,
		vote.ID,
		vote.UserID,
		vote.PollID,
		vote.PollOptionID,
		vote.CreatedAt,
	)

	return err
}

// ChangeVote changes a user's vote
func (r *pollRepository) ChangeVote(ctx context.Context, userID, pollID, newOptionID string) error {
	query := `
		UPDATE user_polls
		SET poll_option_id = $3
		WHERE user_id = $1 AND poll_id = $2 AND deleted_at IS NULL
	`

	result, err := r.db.Pool.Exec(ctx, query, userID, pollID, newOptionID)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("vote not found")
	}

	return nil
}

// DeleteVote deletes a user's vote
func (r *pollRepository) DeleteVote(ctx context.Context, userID, pollID string) error {
	query := `
		UPDATE user_polls
		SET deleted_at = $3
		WHERE user_id = $1 AND poll_id = $2 AND deleted_at IS NULL
	`

	_, err := r.db.Pool.Exec(ctx, query, userID, pollID, time.Now())
	return err
}

// GetUserVote gets a user's vote on a poll
func (r *pollRepository) GetUserVote(ctx context.Context, userID, pollID string) (*models.UserPoll, error) {
	query := `
		SELECT id, user_id, poll_id, poll_option_id, created_at
		FROM user_polls
		WHERE user_id = $1 AND poll_id = $2 AND deleted_at IS NULL
	`

	vote := &models.UserPoll{}
	err := r.db.Pool.QueryRow(ctx, query, userID, pollID).Scan(
		&vote.ID,
		&vote.UserID,
		&vote.PollID,
		&vote.PollOptionID,
		&vote.CreatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, nil // Not an error, user just hasn't voted
	}

	return vote, err
}

// HasUserVoted checks if a user has voted on a poll
func (r *pollRepository) HasUserVoted(ctx context.Context, userID, pollID string) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM user_polls
			WHERE user_id = $1 AND poll_id = $2 AND deleted_at IS NULL
		)
	`

	var exists bool
	err := r.db.Pool.QueryRow(ctx, query, userID, pollID).Scan(&exists)
	return exists, err
}
