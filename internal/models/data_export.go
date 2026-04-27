package models

import "time"

// UserDataExport is the GDPR Article 20 (data portability) payload returned
// by GET /profile/export. It bundles the user-owned data the platform stores
// so the user can download a copy in a structured machine-readable format.
//
// Caps: posts/comments/follows lists are capped at 5000 each — beyond that
// the export becomes an offline job, not a request-cycle JSON. Counts in
// the response let the client warn the user when truncation happened.
type UserDataExport struct {
	GeneratedAt time.Time            `json:"generated_at"`
	Format      string               `json:"format"` // "json"
	Version     string               `json:"version"`
	Profile     *FullProfileResponse `json:"profile"`

	Posts    []*Post        `json:"posts"`
	Comments []*PostComment `json:"comments"`

	// Relationships are flattened to user-id lists so the JSON stays small.
	// The user can re-resolve any user-id via the public profile endpoint.
	FollowerIDs  []string `json:"follower_ids"`
	FollowingIDs []string `json:"following_ids"`
	BlockedIDs   []string `json:"blocked_ids"`

	BookmarkPostIDs []string `json:"bookmark_post_ids"`

	Counts ExportCounts `json:"counts"`
}

// ExportCounts surfaces totals separate from the (possibly truncated) lists.
type ExportCounts struct {
	Posts     int `json:"posts"`
	Comments  int `json:"comments"`
	Followers int `json:"followers"`
	Following int `json:"following"`
	Blocked   int `json:"blocked"`
	Bookmarks int `json:"bookmarks"`
}
