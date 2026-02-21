package models

// AdminStatistics represents the dashboard statistics for admin
type AdminStatistics struct {
	// User Statistics - Account Status
	TotalActiveAccounts int64 `json:"total_active_accounts"` // Accounts with is_active = true
	DeactivatedAccounts int64 `json:"deactivated_accounts"`  // Accounts with is_active = false
	NewUsersThisMonth   int64 `json:"new_users_this_month"`

	// User Statistics - Login Activity (for active accounts only)
	RecentlyActiveUsers int64 `json:"recently_active_users"` // Logged in last 30 days AND is_active = true
	DormantUsers        int64 `json:"dormant_users"`         // No login in 30+ days AND is_active = true

	// Post Statistics
	TotalPosts        int64         `json:"total_posts"`
	PostsByType       PostTypeStats `json:"posts_by_type"`
	NewPostsThisMonth int64         `json:"new_posts_this_month"`

	// Business Statistics
	TotalBusinesses        int64 `json:"total_businesses"`
	ActiveBusinesses       int64 `json:"active_businesses"`
	NewBusinessesThisMonth int64 `json:"new_businesses_this_month"`

	// Engagement Statistics
	TotalComments  int64 `json:"total_comments"`
	TotalLikes     int64 `json:"total_likes"`
	TotalShares    int64 `json:"total_shares"`
	TotalBookmarks int64 `json:"total_bookmarks"`

	// Activity Statistics
	TotalCategories     int64 `json:"total_categories"`
	TotalPollVotes      int64 `json:"total_poll_votes"`
	TotalEventInterests int64 `json:"total_event_interests"`
	TotalFollows        int64 `json:"total_follows"`

	// Content Moderation
	PendingReports  PendingReports `json:"pending_reports"`
	TotalReports    int64          `json:"total_reports"`
	ResolvedReports int64          `json:"resolved_reports"`
}

// PostTypeStats represents post counts by type
type PostTypeStats struct {
	Feed  int64 `json:"feed"`
	Event int64 `json:"event"`
	Sell  int64 `json:"sell"`
	Pull  int64 `json:"pull"`
}

// PendingReports represents pending reports counts by type
type PendingReports struct {
	Posts     int64 `json:"posts"`
	Comments  int64 `json:"comments"`
	Users     int64 `json:"users"`
	Businesses int64 `json:"businesses"`
	Total     int64 `json:"total"`
}

// AdminUserListItem represents a user in the admin user list
type AdminUserListItem struct {
	ID            string  `json:"id"`
	Email         string  `json:"email"`
	FirstName     *string `json:"first_name"`
	LastName      *string `json:"last_name"`
	EmailVerified bool    `json:"email_verified"`
	PhoneVerified bool    `json:"phone_verified"`
	MFAEnabled    bool    `json:"mfa_enabled"`
	Role          string  `json:"role"`
	IsActive      bool    `json:"is_active"`
	LastLoginAt   *string `json:"last_login_at"`
	CreatedAt     string  `json:"created_at"`
}

// UpdateUserStatusRequest represents a request to update user status
type UpdateUserStatusRequest struct {
	IsActive bool `json:"is_active"`
}

// AdminPostListItem represents a post in the admin post list
type AdminPostListItem struct {
	ID            string  `json:"id"`
	UserID        *string `json:"user_id"`
	UserEmail     *string `json:"user_email"`
	UserName      *string `json:"user_name"`
	BusinessID    *string `json:"business_id"`
	BusinessName  *string `json:"business_name"`
	Type          string  `json:"type"`
	Title         *string `json:"title"`
	Description   *string `json:"description"`
	Visibility    string  `json:"visibility"`
	Status        bool    `json:"status"`
	StartDate     *string `json:"start_date"`
	EndDate       *string `json:"end_date"`
	Attachments   []Photo `json:"attachments"`
	TotalLikes    int64   `json:"total_likes"`
	TotalComments int64   `json:"total_comments"`
	TotalShares   int64   `json:"total_shares"`
	CreatedAt     string  `json:"created_at"`
	UpdatedAt     string  `json:"updated_at"`
}

// AdminReportListItem represents a report in the admin report list
type AdminReportListItem struct {
	ID                 string  `json:"id"`
	ReportType         string  `json:"report_type"` // POST, COMMENT, USER, BUSINESS
	ReporterID         string  `json:"reporter_id"`
	ReporterEmail      *string `json:"reporter_email"`
	ReporterName       *string `json:"reporter_name"`
	ReportedItemID     string  `json:"reported_item_id"`
	ReportedItemInfo   *string `json:"reported_item_info"` // Title/Name of reported item
	Reason             string  `json:"reason"`
	AdditionalComments *string `json:"additional_comments"`
	Status             string  `json:"status"` // PENDING, REVIEWING, RESOLVED, REJECTED
	CreatedAt          string  `json:"created_at"`
	UpdatedAt          string  `json:"updated_at"`
}

// AdminBusinessListItem represents a business in the admin business list
type AdminBusinessListItem struct {
	ID          string  `json:"id"`
	UserID      string  `json:"user_id"`
	OwnerEmail  *string `json:"owner_email"`
	OwnerName   *string `json:"owner_name"`
	Name        string  `json:"name"`
	LicenseNo   *string `json:"license_no"`
	Email       *string `json:"email"`
	PhoneNumber *string `json:"phone_number"`
	Province    *string `json:"province"`
	District    *string `json:"district"`
	Status      bool    `json:"status"`
	TotalViews  int64   `json:"total_views"`
	TotalFollow int64   `json:"total_follow"`
	TotalPosts  int64   `json:"total_posts"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

// UpdateBusinessStatusRequest represents a request to update business status
type UpdateBusinessStatusRequest struct {
	Status bool `json:"status"`
}

// UpdatePostStatusRequest represents a request to update post status
type UpdatePostStatusRequest struct {
	Status bool `json:"status"`
}

// AdminUpdateUserRequest represents an admin request to update user
type AdminUpdateUserRequest struct {
	FirstName     *string `json:"first_name"`
	LastName      *string `json:"last_name"`
	Email         *string `json:"email" validate:"omitempty,email"`
	Role          *string `json:"role" validate:"omitempty,oneof=user admin moderator"`
	EmailVerified *bool   `json:"email_verified"`
	PhoneVerified *bool   `json:"phone_verified"`
	IsActive      *bool   `json:"is_active"`
	MfaEnabled    *bool   `json:"mfa_enabled"`
}

// AdminUpdatePostRequest represents an admin request to update post
type AdminUpdatePostRequest struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	Visibility  *string `json:"visibility" validate:"omitempty,oneof=PUBLIC FRIENDS PRIVATE"`
	Type        *string `json:"type" validate:"omitempty,oneof=FEED EVENT SELL PULL"`
	Status      *bool   `json:"status"`
	StartDate   *string `json:"start_date"`
	EndDate     *string `json:"end_date"`
}

// AdminUpdateBusinessRequest represents an admin request to update business
type AdminUpdateBusinessRequest struct {
	Name        *string `json:"name"`
	LicenseNo   *string `json:"license_no"`
	Email       *string `json:"email" validate:"omitempty,email"`
	PhoneNumber *string `json:"phone_number"`
	Province    *string `json:"province"`
	District    *string `json:"district"`
}

// SellStatistics represents statistics for SELL type posts
type SellStatistics struct {
	TotalSellPosts int     `json:"total_sell_posts"` // Total number of SELL posts
	TotalSold      int     `json:"total_sold"`       // Number of sold items
	TotalActive    int     `json:"total_active"`     // Number of active (not sold, not expired) items
	TotalExpired   int     `json:"total_expired"`    // Number of expired items
	TotalRevenue   float64 `json:"total_revenue"`    // Sum of prices for sold items
	AveragePrice   float64 `json:"average_price"`    // Average price of all SELL posts
}
