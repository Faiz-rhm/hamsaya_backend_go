// Command backfill-notifications creates notification rows for existing post likes and
// comments that happened before the notification system was fixed (e.g. context cancellation).
// Run once: go run cmd/backfill-notifications/main.go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/hamsaya/backend/config"
	"github.com/hamsaya/backend/pkg/database"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()
	db, err := database.New(&cfg.Database)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// Backfill LIKE notifications: post_likes where liker != post owner
	likeQuery := `
		INSERT INTO notifications (id, user_id, type, title, message, data, read, created_at)
		SELECT
			uuid_generate_v4(),
			p.user_id,
			'LIKE',
			COALESCE(TRIM(pr.first_name || ' ' || pr.last_name), 'Someone') || ' liked your post',
			COALESCE(TRIM(pr.first_name || ' ' || pr.last_name), 'Someone') || ' liked your post',
			jsonb_build_object(
				'actor_id', pl.user_id,
				'actor_name', COALESCE(TRIM(pr.first_name || ' ' || pr.last_name), 'Someone'),
				'actor_avatar', pr.avatar,
				'post_id', p.id
			),
			false,
			pl.created_at
		FROM post_likes pl
		JOIN posts p ON p.id = pl.post_id AND p.deleted_at IS NULL AND p.user_id IS NOT NULL AND p.user_id != pl.user_id
		LEFT JOIN profiles pr ON pr.id = pl.user_id
		WHERE NOT EXISTS (
			SELECT 1 FROM notifications n
			WHERE n.user_id = p.user_id AND n.type = 'LIKE'
			  AND n.data->>'post_id' = p.id::text AND n.data->>'actor_id' = pl.user_id::text
		)
	`
	likeRes, err := db.Pool.Exec(ctx, likeQuery)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to backfill LIKE notifications: %v\n", err)
		os.Exit(1)
	}
	likeCount := likeRes.RowsAffected()
	fmt.Printf("Backfilled %d LIKE notification(s).\n", likeCount)

	// Backfill COMMENT notifications: post_comments where commenter != post owner
	commentRows, err := db.Pool.Query(ctx, `
		SELECT pc.post_id, pc.user_id AS actor_id, p.user_id AS recipient_id, pc.created_at,
			COALESCE(TRIM(pr.first_name || ' ' || pr.last_name), 'Someone') AS actor_name,
			pr.avatar AS actor_avatar
		FROM post_comments pc
		JOIN posts p ON p.id = pc.post_id AND p.deleted_at IS NULL AND p.user_id IS NOT NULL AND p.user_id != pc.user_id
		LEFT JOIN profiles pr ON pr.id = pc.user_id
		WHERE pc.deleted_at IS NULL
		ORDER BY pc.created_at ASC
	`)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to query comments: %v\n", err)
		os.Exit(1)
	}
	defer commentRows.Close()

	var postID, actorID, recipientID string
	var createdAt time.Time
	var actorName string
	var actorAvatar interface{}

	commentCount := int64(0)
	for commentRows.Next() {
		if err := commentRows.Scan(&postID, &actorID, &recipientID, &createdAt, &actorName, &actorAvatar); err != nil {
			fmt.Fprintf(os.Stderr, "Scan comment row: %v\n", err)
			continue
		}

		// Skip if notification already exists
		var exists bool
		err := db.Pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM notifications n
				WHERE n.user_id = $1 AND n.type = 'COMMENT'
				  AND n.data->>'post_id' = $2 AND n.data->>'actor_id' = $3
			)
		`, recipientID, postID, actorID).Scan(&exists)
		if err != nil || exists {
			continue
		}

		data := map[string]interface{}{
			"actor_id":     actorID,
			"actor_name":   actorName,
			"actor_avatar": actorAvatar,
			"post_id":      postID,
		}
		dataJSON, _ := json.Marshal(data)
		title := actorName + " commented on your post"
		_, err = db.Pool.Exec(ctx, `
			INSERT INTO notifications (id, user_id, type, title, message, data, read, created_at)
			VALUES ($1, $2, 'COMMENT', $3, $4, $5, false, $6)
		`, uuid.New().String(), recipientID, title, title, dataJSON, createdAt)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Insert comment notification: %v\n", err)
			continue
		}
		commentCount++
	}
	if err := commentRows.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Comment rows error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Backfilled %d COMMENT notification(s).\n", commentCount)

	fmt.Printf("Done. Total: %d like(s) + %d comment(s).\n", likeCount, commentCount)
}
