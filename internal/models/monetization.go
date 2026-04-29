package models

import "time"

// ─── Ads ─────────────────────────────────────────────────────────────────────

// Ad is a single advertiser-submitted placement awaiting review or already
// running. Status transitions are enforced by service-layer rules.
type Ad struct {
	ID              string     `json:"id"`
	AdvertiserID    string     `json:"advertiser_id"`
	AdvertiserEmail string     `json:"advertiser_email,omitempty"`
	AdvertiserName  string     `json:"advertiser_name,omitempty"`
	Title           string     `json:"title"`
	Body            *string    `json:"body,omitempty"`
	ImageURL        *string    `json:"image_url,omitempty"`
	TargetURL       string     `json:"target_url"`
	Status          string     `json:"status"`
	StartAt         *time.Time `json:"start_at,omitempty"`
	EndAt           *time.Time `json:"end_at,omitempty"`
	Impressions     int        `json:"impressions"`
	Clicks          int        `json:"clicks"`
	ReviewedBy      *string    `json:"reviewed_by,omitempty"`
	ReviewedAt      *time.Time `json:"reviewed_at,omitempty"`
	ReviewNote      *string    `json:"review_note,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// AdCreateRequest is the admin payload for creating an ad row. The image is
// uploaded as a multipart file under the "image" field; the rest of the
// fields ride alongside as form values. AdvertiserID identifies the user the
// placement belongs to (super_admin can create on behalf of any user).
type AdCreateRequest struct {
	AdvertiserID string     `form:"advertiser_id" validate:"required,uuid"`
	Title        string     `form:"title"         validate:"required,min=2,max=120"`
	Body         *string    `form:"body"          validate:"omitempty,max=2000"`
	TargetURL    string     `form:"target_url"    validate:"required,url"`
	StartAt      *time.Time `form:"start_at"`
	EndAt        *time.Time `form:"end_at"`
	// AutoApprove flips the new row from PENDING to ACTIVE on creation —
	// admin convenience so a super_admin doesn't need a second click.
	AutoApprove bool `form:"auto_approve"`
}

// AdReviewRequest is the admin payload for approving / rejecting an ad.
// `note` is optional and surfaced back to the advertiser. Status transitions:
//   - PENDING → APPROVED + start/end window may be set, status flips to ACTIVE
//     once start_at <= now.
//   - PENDING → REJECTED + note required (UI enforces).
type AdReviewRequest struct {
	Note    *string    `json:"note,omitempty"     validate:"omitempty,max=500"`
	StartAt *time.Time `json:"start_at,omitempty"`
	EndAt   *time.Time `json:"end_at,omitempty"`
}

// ─── Credits ─────────────────────────────────────────────────────────────────

// CreditBalance is the current balance for a user. Returned in the admin
// list view alongside the user identity columns.
type CreditBalance struct {
	UserID    string    `json:"user_id"`
	Email     string    `json:"email,omitempty"`
	FullName  string    `json:"full_name,omitempty"`
	Balance   int       `json:"balance"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CreditTransaction is a single ledger entry. Positive = credit, negative =
// debit. type narrows the reason category; reason is a short admin-supplied
// string when applicable.
type CreditTransaction struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	Amount     int       `json:"amount"`
	Type       string    `json:"type"`
	Reason     *string   `json:"reason,omitempty"`
	Note       *string   `json:"note,omitempty"`
	AdminID    *string   `json:"admin_id,omitempty"`
	AdminEmail *string   `json:"admin_email,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// AdjustCreditsRequest is the admin payload to debit/credit a user. `amount`
// can be negative (debit) or positive (credit). The service enforces that
// debits don't drive balance below zero.
type AdjustCreditsRequest struct {
	Amount int     `json:"amount"  validate:"required,min=-100000,max=100000"`
	Reason string  `json:"reason"  validate:"required,min=2,max=120"`
	Note   *string `json:"note,omitempty" validate:"omitempty,max=500"`
}

// CreditsUserDetail is the admin detail view: balance + recent transactions.
type CreditsUserDetail struct {
	Balance      CreditBalance       `json:"balance"`
	Transactions []CreditTransaction `json:"transactions"`
}

// ─── Boosts ──────────────────────────────────────────────────────────────────

// Boost is a single post-promotion window. The admin panel surfaces these so
// operators can spot abuse and cancel ACTIVE rows. Cancellation by an admin
// records cancelled_by + cancelled_at + cancel_reason.
type Boost struct {
	ID            string     `json:"id"`
	PostID        string     `json:"post_id"`
	PostTitle     string     `json:"post_title,omitempty"`
	UserID        string     `json:"user_id"`
	UserEmail     string     `json:"user_email,omitempty"`
	Status        string     `json:"status"`
	StartedAt     time.Time  `json:"started_at"`
	ExpiresAt     time.Time  `json:"expires_at"`
	CreditsSpent  int        `json:"credits_spent"`
	CancelledBy   *string    `json:"cancelled_by,omitempty"`
	CancelledAt   *time.Time `json:"cancelled_at,omitempty"`
	CancelReason  *string    `json:"cancel_reason,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

// CancelBoostRequest is the admin payload to terminate an ACTIVE boost.
type CancelBoostRequest struct {
	Reason string `json:"reason" validate:"required,min=2,max=500"`
}
