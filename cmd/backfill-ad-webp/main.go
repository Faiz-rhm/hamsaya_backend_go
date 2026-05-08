// Command backfill-ad-webp re-encodes existing ad images to WebP and updates
// the ads.image_url column. New uploads are already WebP via the
// ImageTypeAd path; this one-shot covers rows uploaded before that fix.
//
// Usage:
//
//	go run ./cmd/backfill-ad-webp           # apply changes
//	go run ./cmd/backfill-ad-webp -dry-run  # log what would change, no writes
//
// Idempotent: rows whose image_url already ends in .webp are skipped.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/zap"

	"github.com/hamsaya/backend/config"
	"github.com/hamsaya/backend/pkg/database"
	"github.com/hamsaya/backend/pkg/storage"
)

func main() {
	dryRun := flag.Bool("dry-run", false, "log changes only; do not transcode or update DB")
	flag.Parse()

	logger, _ := zap.NewProduction()
	defer func() { _ = logger.Sync() }()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config load: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()
	db, err := database.New(&cfg.Database)
	if err != nil {
		fmt.Fprintf(os.Stderr, "db connect: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	storageCfg := &storage.Config{
		Endpoint:   cfg.Storage.Endpoint,
		AccessKey:  cfg.Storage.AccessKey,
		SecretKey:  cfg.Storage.SecretKey,
		BucketName: cfg.Storage.BucketName,
		UseSSL:     cfg.Storage.UseSSL,
		Region:     cfg.Storage.Region,
		CDNURL:     cfg.Storage.CDNURL,
	}
	if storageCfg.Endpoint == "" {
		fmt.Fprintln(os.Stderr, "storage not configured (STORAGE_ENDPOINT empty)")
		os.Exit(1)
	}

	client, err := storage.NewClient(storageCfg, logger)
	if err != nil {
		fmt.Fprintf(os.Stderr, "storage client: %v\n", err)
		os.Exit(1)
	}

	rows, err := db.Pool.Query(ctx, `
		SELECT id, image_url
		FROM ads
		WHERE image_url IS NOT NULL
		  AND image_url <> ''
		  AND image_url NOT ILIKE '%.webp'
	`)
	if err != nil {
		fmt.Fprintf(os.Stderr, "select ads: %v\n", err)
		os.Exit(1)
	}
	defer rows.Close()

	type pending struct {
		id, oldURL, oldKey, newKey, newURL string
	}
	var todo []pending
	for rows.Next() {
		var id, url string
		if err := rows.Scan(&id, &url); err != nil {
			fmt.Fprintf(os.Stderr, "scan: %v\n", err)
			continue
		}
		oldKey := client.KeyFromURL(url)
		if oldKey == "" {
			fmt.Fprintf(os.Stderr, "ad %s: cannot parse key from %q — skipped\n", id, url)
			continue
		}
		// Same folder + uuid, just swap the extension to .webp.
		ext := filepath.Ext(oldKey)
		base := strings.TrimSuffix(oldKey, ext)
		newKey := base + ".webp"
		todo = append(todo, pending{
			id:     id,
			oldURL: url,
			oldKey: oldKey,
			newKey: newKey,
			newURL: client.PublicURL(newKey),
		})
	}
	if err := rows.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "rows iter: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("ads to process: %d (dry-run=%v)\n", len(todo), *dryRun)

	var ok, fail int
	for _, p := range todo {
		fmt.Printf("→ %s  %s → %s\n", p.id, p.oldKey, p.newKey)
		if *dryRun {
			ok++
			continue
		}
		if err := client.Transcode(ctx, p.oldKey, p.newKey, "webp", 90); err != nil {
			fmt.Fprintf(os.Stderr, "  transcode failed: %v\n", err)
			fail++
			continue
		}
		if _, err := db.Pool.Exec(ctx,
			`UPDATE ads SET image_url = $1, updated_at = NOW() WHERE id = $2`,
			p.newURL, p.id,
		); err != nil {
			fmt.Fprintf(os.Stderr, "  db update failed: %v\n", err)
			fail++
			continue
		}
		ok++
	}
	fmt.Printf("done: ok=%d fail=%d total=%d\n", ok, fail, len(todo))
}
