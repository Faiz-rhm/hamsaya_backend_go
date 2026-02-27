package repositories

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/pkg/database"
	"github.com/jackc/pgx/v5"
)

// CommentRepository defines the interface for comment operations
type CommentRepository interface {
	// Comment CRUD
	Create(ctx context.Context, comment *models.PostComment) error
	GetByID(ctx context.Context, commentID string) (*models.PostComment, error)
	Update(ctx context.Context, comment *models.PostComment) error
	Delete(ctx context.Context, commentID string) error

	// Comment queries
	GetByPostID(ctx context.Context, postID string, limit, offset int) ([]*models.PostComment, error)
	GetReplies(ctx context.Context, parentCommentID string, limit, offset int) ([]*models.PostComment, error)
	CountByPostID(ctx context.Context, postID string) (int, error)

	// Comment attachments
	CreateAttachment(ctx context.Context, attachment *models.CommentAttachment) error
	GetAttachmentsByCommentID(ctx context.Context, commentID string) ([]*models.CommentAttachment, error)
	DeleteAttachment(ctx context.Context, attachmentID string) error

	// Comment likes
	LikeComment(ctx context.Context, userID, commentID string) error
	UnlikeComment(ctx context.Context, userID, commentID string) error
	IsLikedByUser(ctx context.Context, userID, commentID string) (bool, error)
	GetCommentLikes(ctx context.Context, commentID string, limit, offset int) ([]*models.CommentLike, error)
}

type commentRepository struct {
	db *database.DB
}

// NewCommentRepository creates a new comment repository
func NewCommentRepository(db *database.DB) CommentRepository {
	return &commentRepository{db: db}
}

// Create creates a new comment
func (r *commentRepository) Create(ctx context.Context, comment *models.PostComment) error {
	query := `
		INSERT INTO post_comments (
			id, post_id, user_id, business_id, parent_comment_id, text, location,
			total_likes, total_replies, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
		)
	`

	_, err := r.db.Pool.Exec(ctx, query,
		comment.ID,
		comment.PostID,
		comment.UserID,
		comment.BusinessID,
		comment.ParentCommentID,
		comment.Text,
		comment.Location,
		comment.TotalLikes,
		comment.TotalReplies,
		comment.CreatedAt,
		comment.UpdatedAt,
	)

	return err
}

// GetByID gets a comment by ID
func (r *commentRepository) GetByID(ctx context.Context, commentID string) (*models.PostComment, error) {
	query := `
		SELECT
			id, post_id, user_id, business_id, parent_comment_id, text, location,
			total_likes, total_replies, created_at, updated_at, deleted_at
		FROM post_comments
		WHERE id = $1 AND deleted_at IS NULL
	`

	comment := &models.PostComment{}
	err := r.db.Pool.QueryRow(ctx, query, commentID).Scan(
		&comment.ID,
		&comment.PostID,
		&comment.UserID,
		&comment.BusinessID,
		&comment.ParentCommentID,
		&comment.Text,
		&comment.Location,
		&comment.TotalLikes,
		&comment.TotalReplies,
		&comment.CreatedAt,
		&comment.UpdatedAt,
		&comment.DeletedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("comment not found")
	}

	return comment, err
}

// Update updates a comment
func (r *commentRepository) Update(ctx context.Context, comment *models.PostComment) error {
	query := `
		UPDATE post_comments SET
			text = $2,
			updated_at = $3
		WHERE id = $1 AND deleted_at IS NULL
	`

	_, err := r.db.Pool.Exec(ctx, query,
		comment.ID,
		comment.Text,
		time.Now(),
	)

	return err
}

// Delete soft deletes a comment
func (r *commentRepository) Delete(ctx context.Context, commentID string) error {
	query := `
		UPDATE post_comments
		SET deleted_at = $2
		WHERE id = $1 AND deleted_at IS NULL
	`

	_, err := r.db.Pool.Exec(ctx, query, commentID, time.Now())
	return err
}

// GetByPostID gets comments by post ID (top-level comments only)
func (r *commentRepository) GetByPostID(ctx context.Context, postID string, limit, offset int) ([]*models.PostComment, error) {
	query := `
		SELECT
			id, post_id, user_id, business_id, parent_comment_id, text, location,
			total_likes, total_replies, created_at, updated_at, deleted_at
		FROM post_comments
		WHERE post_id = $1 AND parent_comment_id IS NULL AND deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	return r.queryComments(ctx, query, postID, limit, offset)
}

// GetReplies gets replies to a comment
func (r *commentRepository) GetReplies(ctx context.Context, parentCommentID string, limit, offset int) ([]*models.PostComment, error) {
	query := `
		SELECT
			id, post_id, user_id, business_id, parent_comment_id, text, location,
			total_likes, total_replies, created_at, updated_at, deleted_at
		FROM post_comments
		WHERE parent_comment_id = $1 AND deleted_at IS NULL
		ORDER BY created_at ASC
		LIMIT $2 OFFSET $3
	`

	return r.queryComments(ctx, query, parentCommentID, limit, offset)
}

// CountByPostID counts comments by post ID
func (r *commentRepository) CountByPostID(ctx context.Context, postID string) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM post_comments
		WHERE post_id = $1 AND deleted_at IS NULL
	`

	var count int
	err := r.db.Pool.QueryRow(ctx, query, postID).Scan(&count)
	return count, err
}

// CreateAttachment creates a comment attachment
func (r *commentRepository) CreateAttachment(ctx context.Context, attachment *models.CommentAttachment) error {
	query := `
		INSERT INTO comment_attachments (id, comment_id, photo, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
	`

	_, err := r.db.Pool.Exec(ctx, query,
		attachment.ID,
		attachment.CommentID,
		attachment.Photo,
		attachment.CreatedAt,
		attachment.UpdatedAt,
	)

	return err
}

// GetAttachmentsByCommentID gets all attachments for a comment
func (r *commentRepository) GetAttachmentsByCommentID(ctx context.Context, commentID string) ([]*models.CommentAttachment, error) {
	query := `
		SELECT id, comment_id, photo, created_at, updated_at
		FROM comment_attachments
		WHERE comment_id = $1 AND deleted_at IS NULL
		ORDER BY created_at ASC
	`

	rows, err := r.db.Pool.Query(ctx, query, commentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var attachments []*models.CommentAttachment
	for rows.Next() {
		attachment := &models.CommentAttachment{}
		err := rows.Scan(
			&attachment.ID,
			&attachment.CommentID,
			&attachment.Photo,
			&attachment.CreatedAt,
			&attachment.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		attachments = append(attachments, attachment)
	}

	return attachments, rows.Err()
}

// DeleteAttachment soft deletes a comment attachment
func (r *commentRepository) DeleteAttachment(ctx context.Context, attachmentID string) error {
	query := `
		UPDATE comment_attachments
		SET deleted_at = $2
		WHERE id = $1 AND deleted_at IS NULL
	`

	_, err := r.db.Pool.Exec(ctx, query, attachmentID, time.Now())
	return err
}

// LikeComment likes a comment (idempotent)
func (r *commentRepository) LikeComment(ctx context.Context, userID, commentID string) error {
	query := `
		INSERT INTO post_comment_likes (id, user_id, comment_id, created_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id, comment_id) DO NOTHING
	`

	_, err := r.db.Pool.Exec(ctx, query,
		uuid.New().String(),
		userID,
		commentID,
		time.Now(),
	)

	return err
}

// UnlikeComment unlikes a comment
func (r *commentRepository) UnlikeComment(ctx context.Context, userID, commentID string) error {
	query := `
		DELETE FROM post_comment_likes
		WHERE user_id = $1 AND comment_id = $2
	`

	_, err := r.db.Pool.Exec(ctx, query, userID, commentID)
	return err
}

// IsLikedByUser checks if a comment is liked by a user
func (r *commentRepository) IsLikedByUser(ctx context.Context, userID, commentID string) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM post_comment_likes
			WHERE user_id = $1 AND comment_id = $2
		)
	`

	var exists bool
	err := r.db.Pool.QueryRow(ctx, query, userID, commentID).Scan(&exists)
	return exists, err
}

// GetCommentLikes gets all likes for a comment
func (r *commentRepository) GetCommentLikes(ctx context.Context, commentID string, limit, offset int) ([]*models.CommentLike, error) {
	query := `
		SELECT id, user_id, comment_id, created_at
		FROM post_comment_likes
		WHERE comment_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.Pool.Query(ctx, query, commentID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var likes []*models.CommentLike
	for rows.Next() {
		like := &models.CommentLike{}
		err := rows.Scan(&like.ID, &like.UserID, &like.CommentID, &like.CreatedAt)
		if err != nil {
			return nil, err
		}
		likes = append(likes, like)
	}

	return likes, rows.Err()
}

// queryComments is a helper function to query comments
func (r *commentRepository) queryComments(ctx context.Context, query string, args ...interface{}) ([]*models.PostComment, error) {
	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var comments []*models.PostComment
	for rows.Next() {
		comment := &models.PostComment{}
		err := rows.Scan(
			&comment.ID,
			&comment.PostID,
			&comment.UserID,
			&comment.BusinessID,
			&comment.ParentCommentID,
			&comment.Text,
			&comment.Location,
			&comment.TotalLikes,
			&comment.TotalReplies,
			&comment.CreatedAt,
			&comment.UpdatedAt,
			&comment.DeletedAt,
		)
		if err != nil {
			return nil, err
		}
		comments = append(comments, comment)
	}

	return comments, rows.Err()
}
