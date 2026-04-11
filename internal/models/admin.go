package models

import "time"

// DashboardStats contains aggregate statistics for the admin dashboard
type DashboardStats struct {
	TotalUsers         int64 `json:"total_users"`
	NewUsersToday      int64 `json:"new_users_today"`
	NewUsersWeek       int64 `json:"new_users_week"`
	NewUsersMonth      int64 `json:"new_users_month"`
	SuspendedUsers     int64 `json:"suspended_users"`
	TotalPosts         int64 `json:"total_posts"`
	TotalFeedPosts     int64 `json:"total_feed_posts"`
	TotalEventPosts    int64 `json:"total_event_posts"`
	TotalSellPosts     int64 `json:"total_sell_posts"`
	TotalPollPosts     int64 `json:"total_poll_posts"`
	TotalBusinesses    int64 `json:"total_businesses"`
	ActiveBusinesses   int64 `json:"active_businesses"`
	PendingBusinesses  int64 `json:"pending_businesses"`
	NewBusinessesWeek  int64 `json:"new_businesses_week"`
	PendingReports     int64 `json:"pending_reports"`
	ResolvedReports    int64 `json:"resolved_reports"`
	TotalComments      int64 `json:"total_comments"`
	TotalLikes         int64 `json:"total_likes"`
}

// TimeSeriesData represents a data point in time series analytics
type TimeSeriesData struct {
	Date  string `json:"date"`
	Count int64  `json:"count"`
}

// UserAnalytics contains user growth and activity analytics
type UserAnalytics struct {
	GrowthData      []TimeSeriesData `json:"growth_data"`
	ActiveUsersData []TimeSeriesData `json:"active_users_data"`
	TotalUsers      int64            `json:"total_users"`
	ActiveUsers     int64            `json:"active_users"`
}

// PostAnalytics contains post activity analytics
type PostAnalytics struct {
	PostsOverTime   []TimeSeriesData    `json:"posts_over_time"`
	PostsByType     []PostTypeCount     `json:"posts_by_type"`
	TotalPosts      int64               `json:"total_posts"`
}

// PostTypeCount represents count of posts by type
type PostTypeCount struct {
	Type  string `json:"type"`
	Count int64  `json:"count"`
}

// EngagementAnalytics contains engagement metrics
type EngagementAnalytics struct {
	LikesOverTime    []TimeSeriesData `json:"likes_over_time"`
	CommentsOverTime []TimeSeriesData `json:"comments_over_time"`
	SharesOverTime   []TimeSeriesData `json:"shares_over_time"`
	TotalLikes       int64            `json:"total_likes"`
	TotalComments    int64            `json:"total_comments"`
	TotalShares      int64            `json:"total_shares"`
}

// AdminUserFilter contains filters for listing users in admin panel
type AdminUserFilter struct {
	Search   string   `form:"search"`
	Role     string   `form:"role"`
	Status   string   `form:"status"`
	Province string   `form:"province"`
	SortBy   string   `form:"sort_by"`
	SortDir  string   `form:"sort_dir"`
	Page     int      `form:"page"`
	Limit    int      `form:"limit"`
}

// AdminUserResponse is the user data returned in admin API
type AdminUserResponse struct {
	ID            string     `json:"id"`
	Email         string     `json:"email"`
	Phone         *string    `json:"phone,omitempty"`
	EmailVerified bool       `json:"email_verified"`
	MFAEnabled    bool       `json:"mfa_enabled"`
	Role          UserRole   `json:"role"`
	FirstName     *string    `json:"first_name,omitempty"`
	LastName      *string    `json:"last_name,omitempty"`
	Avatar        *Photo     `json:"avatar,omitempty"`
	Cover         *Photo     `json:"cover,omitempty"`
	Country       *string    `json:"country,omitempty"`
	Province      *string    `json:"province,omitempty"`
	District      *string    `json:"district,omitempty"`
	Neighborhood  *string    `json:"neighborhood,omitempty"`
	Latitude      *float64   `json:"latitude,omitempty"`
	Longitude     *float64   `json:"longitude,omitempty"`
	IsComplete    bool       `json:"is_complete"`
	OAuthProvider *string    `json:"oauth_provider,omitempty"`
	IsSuspended   bool       `json:"is_suspended"`
	LockedUntil   *time.Time `json:"locked_until,omitempty"`
	LastLoginAt   *time.Time `json:"last_login_at,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	PostsCount     int64     `json:"posts_count"`
	FollowersCount int64     `json:"followers_count"`
	FollowingCount int64     `json:"following_count"`
}

// AdminUserDetailResponse is the full user detail returned in admin API
type AdminUserDetailResponse struct {
	AdminUserResponse
	Bio            *string                  `json:"bio,omitempty"`
	BusinessCount  int64                    `json:"business_count"`
	RecentPosts    []AdminPostResponse      `json:"recent_posts"`
	Businesses     []AdminBusinessResponse  `json:"businesses"`
}

// AdminPostFilter contains filters for listing posts in admin panel
type AdminPostFilter struct {
	Search    string `form:"search"`
	Type      string `form:"type"`
	Status    string `form:"status"`
	UserID    string `form:"user_id"`
	Reported  bool   `form:"reported"`
	SortBy    string `form:"sort_by"`
	SortDir   string `form:"sort_dir"`
	Page      int    `form:"page"`
	Limit     int    `form:"limit"`
}

// AdminPostResponse is the post data returned in admin API
type AdminPostResponse struct {
	ID            string     `json:"id"`
	Type          string     `json:"type"`
	Title         *string    `json:"title,omitempty"`
	Description   *string    `json:"description,omitempty"`
	Status        string     `json:"status"`
	AuthorID      string     `json:"author_id"`
	AuthorEmail   string     `json:"author_email"`
	AuthorName    string     `json:"author_name"`
	TotalLikes    int64      `json:"total_likes"`
	TotalComments int64      `json:"total_comments"`
	TotalShares   int64      `json:"total_shares"`
	ReportCount   int64      `json:"report_count"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// AdminPostDetailResponse is the full post detail returned in admin API
type AdminPostDetailResponse struct {
	ID            string                    `json:"id"`
	Type          string                    `json:"type"`
	Title         *string                   `json:"title,omitempty"`
	Description   *string                   `json:"description,omitempty"`
	Status        string                    `json:"status"`
	Visibility    string                    `json:"visibility"`
	AuthorID      string                    `json:"author_id"`
	AuthorEmail   string                    `json:"author_email"`
	AuthorName    string                    `json:"author_name"`
	AuthorAvatar  *Photo                    `json:"author_avatar,omitempty"`
	BusinessID    *string                   `json:"business_id,omitempty"`
	BusinessName  *string                   `json:"business_name,omitempty"`
	CategoryID    *string                   `json:"category_id,omitempty"`
	CategoryName  *string                   `json:"category_name,omitempty"`

	// Sell-specific
	Currency   *string  `json:"currency,omitempty"`
	Price      *float64 `json:"price,omitempty"`
	Discount   *float64 `json:"discount,omitempty"`
	Free       bool     `json:"free"`
	Sold       bool     `json:"sold"`
	IsPromoted bool     `json:"is_promoted"`
	ContactNo  *string  `json:"contact_no,omitempty"`

	// Event-specific
	StartDate       *time.Time `json:"start_date,omitempty"`
	EndDate         *time.Time `json:"end_date,omitempty"`
	EventState      *string    `json:"event_state,omitempty"`
	InterestedCount int        `json:"interested_count"`
	GoingCount      int        `json:"going_count"`

	// Location
	Country      *string  `json:"country,omitempty"`
	Province     *string  `json:"province,omitempty"`
	District     *string  `json:"district,omitempty"`
	Neighborhood *string  `json:"neighborhood,omitempty"`
	Latitude     *float64 `json:"latitude,omitempty"`
	Longitude    *float64 `json:"longitude,omitempty"`

	// Media
	Attachments []AttachmentResponse `json:"attachments"`

	// Engagement
	TotalLikes    int64 `json:"total_likes"`
	TotalComments int64 `json:"total_comments"`
	TotalShares   int64 `json:"total_shares"`
	ReportCount   int64 `json:"report_count"`

	// Comments
	Comments []AdminPostCommentResponse `json:"comments"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// AdminPostCommentResponse is a comment with its replies for the admin post detail
type AdminPostCommentResponse struct {
	ID          string                     `json:"id"`
	Text        string                     `json:"text"`
	AuthorID    string                     `json:"author_id"`
	AuthorEmail string                     `json:"author_email"`
	AuthorName  string                     `json:"author_name"`
	AuthorAvatar *Photo                    `json:"author_avatar,omitempty"`
	TotalLikes  int64                      `json:"total_likes"`
	TotalReplies int64                     `json:"total_replies"`
	ReportCount int64                      `json:"report_count"`
	Replies     []AdminPostCommentResponse `json:"replies"`
	CreatedAt   time.Time                  `json:"created_at"`
}

// AdminCommentFilter contains filters for listing comments in admin panel
type AdminCommentFilter struct {
	Search    string `form:"search"`
	CommentID string `form:"comment_id"` // fetch single comment by ID
	PostID    string `form:"post_id"`
	UserID    string `form:"user_id"`
	Reported  bool   `form:"reported"`
	SortBy    string `form:"sort_by"`
	SortDir   string `form:"sort_dir"`
	Page      int    `form:"page"`
	Limit     int    `form:"limit"`
}

// AdminCommentResponse is the comment data returned in admin API
type AdminCommentResponse struct {
	ID          string    `json:"id"`
	Content     string    `json:"content"`
	PostID      string    `json:"post_id"`
	PostTitle   *string   `json:"post_title,omitempty"`
	AuthorID    string    `json:"author_id"`
	AuthorEmail string    `json:"author_email"`
	AuthorName  string    `json:"author_name"`
	TotalLikes  int64     `json:"total_likes"`
	ReportCount int64     `json:"report_count"`
	CreatedAt   time.Time `json:"created_at"`
}

// AdminCommentDetailResponse is the full comment detail for admin (includes deleted state)
type AdminCommentDetailResponse struct {
	AdminCommentResponse
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}

// AdminBusinessFilter contains filters for listing businesses in admin panel
type AdminBusinessFilter struct {
	Search   string `form:"search"`
	Status   string `form:"status"`
	Category string `form:"category"`
	Province string `form:"province"`
	SortBy   string `form:"sort_by"`
	SortDir  string `form:"sort_dir"`
	Page     int    `form:"page"`
	Limit    int    `form:"limit"`
}

// AdminBusinessResponse is the business data returned in admin API
type AdminBusinessResponse struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description *string   `json:"description,omitempty"`
	Status      string    `json:"status"`
	OwnerID     string    `json:"owner_id"`
	OwnerEmail  string    `json:"owner_email"`
	OwnerName   string    `json:"owner_name"`
	Avatar      *Photo    `json:"avatar,omitempty"`
	Province    *string   `json:"province,omitempty"`
	TotalFollow int64     `json:"total_follow"`
	TotalViews  int64     `json:"total_views"`
	ReportCount int64     `json:"report_count"`
	CreatedAt   time.Time `json:"created_at"`
}

// AdminBusinessDetailResponse is the full business detail returned in admin API
type AdminBusinessDetailResponse struct {
	ID             string                 `json:"id"`
	Name           string                 `json:"name"`
	LicenseNo      *string                `json:"license_no,omitempty"`
	Description    *string                `json:"description,omitempty"`
	Address        *string                `json:"address,omitempty"`
	PhoneNumber    *string                `json:"phone_number,omitempty"`
	Email          *string                `json:"email,omitempty"`
	Website        *string                `json:"website,omitempty"`
	Avatar         *Photo                 `json:"avatar,omitempty"`
	AvatarColor    *string                `json:"avatar_color,omitempty"`
	Cover          *Photo                 `json:"cover,omitempty"`
	Status         string                 `json:"status"`
	AdditionalInfo *string                `json:"additional_info,omitempty"`
	Country        *string                `json:"country,omitempty"`
	Province       *string                `json:"province,omitempty"`
	District       *string                `json:"district,omitempty"`
	Neighborhood   *string                `json:"neighborhood,omitempty"`
	Latitude       *float64               `json:"latitude,omitempty"`
	Longitude      *float64               `json:"longitude,omitempty"`
	ShowLocation   bool                   `json:"show_location"`
	OwnerID        string                 `json:"owner_id"`
	OwnerEmail     string                 `json:"owner_email"`
	OwnerName      string                 `json:"owner_name"`
	OwnerAvatar    *Photo                 `json:"owner_avatar,omitempty"`
	TotalFollow    int64                  `json:"total_follow"`
	TotalViews     int64                  `json:"total_views"`
	TotalPosts     int64                  `json:"total_posts"`
	ReportCount    int64                  `json:"report_count"`
	Hours          []AdminBusinessHour    `json:"hours"`
	Categories     []string               `json:"categories"`
	Gallery        []AttachmentResponse   `json:"gallery"`
	RecentPosts    []AdminPostResponse    `json:"recent_posts"`
	CreatedAt      time.Time              `json:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at"`
}

// AdminBusinessHour represents business hours for a day
type AdminBusinessHour struct {
	Day       string `json:"day"`
	OpenTime  string `json:"open_time"`
	CloseTime string `json:"close_time"`
	IsClosed  bool   `json:"is_closed"`
}

// AdminReportFilter contains filters for listing reports in admin panel
type AdminReportFilter struct {
	PostID     string `form:"post_id"`
	CommentID  string `form:"comment_id"`  // filter by comment (for comment reports)
	UserID     string `form:"user_id"`     // filter by reported user (for user reports)
	BusinessID string `form:"business_id"` // filter by business (for business reports)
	Status     string `form:"status"`
	SortBy     string `form:"sort_by"`
	SortDir    string `form:"sort_dir"`
	Page       int    `form:"page"`
	Limit      int    `form:"limit"`
}

// AdminPostReportResponse is the post report data for admin API
type AdminPostReportResponse struct {
	ID                 string    `json:"id"`
	PostID             string    `json:"post_id"`
	PostTitle          *string   `json:"post_title,omitempty"`
	PostStatus         string    `json:"post_status"` // ACTIVE, HIDDEN, DELETED
	PostAuthorID       string    `json:"post_author_id"`
	PostAuthorEmail    string    `json:"post_author_email"`
	ReporterID         string    `json:"reporter_id"`
	ReporterEmail      string    `json:"reporter_email"`
	Reason             string    `json:"reason"`
	AdditionalComments *string   `json:"additional_comments,omitempty"`
	Status             string    `json:"status"`
	CreatedAt          time.Time `json:"created_at"`
}

// AdminCommentReportResponse is the comment report data for admin API
type AdminCommentReportResponse struct {
	ID                 string    `json:"id"`
	CommentID          string    `json:"comment_id"`
	PostID             string    `json:"post_id"` // post where the comment was made
	CommentContent     string    `json:"comment_content"`
	CommentAuthorID    string    `json:"comment_author_id"`
	CommentAuthorEmail string    `json:"comment_author_email"`
	CommentHidden      bool      `json:"comment_hidden"` // true if comment is soft-deleted
	ReporterID         string    `json:"reporter_id"`
	ReporterEmail      string    `json:"reporter_email"`
	Reason             string    `json:"reason"`
	AdditionalComments *string   `json:"additional_comments,omitempty"`
	Status             string    `json:"status"`
	CreatedAt          time.Time `json:"created_at"`
}

// AdminUserReportResponse is the user report data for admin API
type AdminUserReportResponse struct {
	ID                    string    `json:"id"`
	ReportedUserID        string    `json:"reported_user_id"`
	ReportedUserEmail     string    `json:"reported_user_email"`
	ReportedUserName      string    `json:"reported_user_name"`
	ReportedUserSuspended bool      `json:"reported_user_suspended"` // true if locked_until > NOW()
	ReporterID            string    `json:"reporter_id"`
	ReporterEmail         string    `json:"reporter_email"`
	Reason                string    `json:"reason"`
	Description           *string   `json:"description,omitempty"`
	Resolved              bool      `json:"resolved"`
	CreatedAt             time.Time `json:"created_at"`
}

// AdminBusinessReportResponse is the business report data for admin API
type AdminBusinessReportResponse struct {
	ID                 string    `json:"id"`
	BusinessID         string    `json:"business_id"`
	BusinessName       string    `json:"business_name"`
	BusinessStatus     string    `json:"business_status"` // ACTIVE, PENDING, SUSPENDED, etc.
	BusinessOwnerID    string    `json:"business_owner_id"`
	BusinessOwnerEmail string    `json:"business_owner_email"`
	ReporterID         string    `json:"reporter_id"`
	ReporterEmail      string    `json:"reporter_email"`
	Reason             string    `json:"reason"`
	AdditionalComments *string   `json:"additional_comments,omitempty"`
	Status             string    `json:"status"`
	CreatedAt          time.Time `json:"created_at"`
}

// UpdateUserRoleRequest is the request to update a user's role
type UpdateUserRoleRequest struct {
	Role string `json:"role" binding:"required,oneof=user admin moderator"`
}

// SuspendUserRequest is the request to suspend a user
type SuspendUserRequest struct {
	Reason string `json:"reason" binding:"required"`
	Days   int    `json:"days" binding:"required,min=1,max=365"`
}

// AdminReportStatusRequest is the request to update a report's status (admin API)
type AdminReportStatusRequest struct {
	Status string `json:"status" binding:"required,oneof=PENDING REVIEWING RESOLVED REJECTED"`
}

// UpdatePostStatusRequest is the request to update a post's status
type UpdatePostStatusRequest struct {
	Status string `json:"status" binding:"required,oneof=ACTIVE HIDDEN DELETED"`
}

// UpdateBusinessStatusRequest is the request to update a business's status
type UpdateBusinessStatusRequest struct {
	Status string `json:"status" binding:"required,oneof=ACTIVE PENDING SUSPENDED REJECTED"`
}

// BroadcastNotificationRequest is the request to send a broadcast notification
type BroadcastNotificationRequest struct {
	Title    string   `json:"title" binding:"required,max=100"`
	Message  string   `json:"message" binding:"required,max=500"`
	Province *string  `json:"province,omitempty"`
	UserIDs  []string `json:"user_ids,omitempty"`
}

// AdminFeedbackResponse is user feedback for admin list
type AdminFeedbackResponse struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	UserEmail  string    `json:"user_email"`
	Rating     int       `json:"rating"` // 1-5
	Type       string    `json:"type"`   // GENERAL, BUG, FEATURE, IMPROVEMENT
	Message    string    `json:"message"`
	AppVersion *string   `json:"app_version,omitempty"`
	DeviceInfo *string   `json:"device_info,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// AdminFeedbackFilter is filter for listing feedback
type AdminFeedbackFilter struct {
	Page   int    `form:"page"`
	Limit  int    `form:"limit"`
	Type   string `form:"type"` // GENERAL, BUG, FEATURE, IMPROVEMENT or empty for all
}


// PaginatedResponse is a generic paginated response
type PaginatedResponse struct {
	Items      interface{} `json:"items"`
	TotalCount int64       `json:"total_count"`
	Page       int         `json:"page"`
	Limit      int         `json:"limit"`
	TotalPages int         `json:"total_pages"`
}
