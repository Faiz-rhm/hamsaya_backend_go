package config

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// parseStringSlice splits a comma-separated string into a slice of strings
func parseStringSlice(s string) []string {
	if s == "" {
		return []string{}
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// getInt32 reads an integer config key and clamps it to int32 range, defending
// against G115 (integer overflow conversion) when an env var supplies a value
// larger than math.MaxInt32 or smaller than math.MinInt32.
func getInt32(key string) int32 {
	v := viper.GetInt(key)
	switch {
	case v > math.MaxInt32:
		return math.MaxInt32
	case v < math.MinInt32:
		return math.MinInt32
	default:
		return int32(v)
	}
}

// Config holds all configuration for the application
type Config struct {
	Server    ServerConfig
	Database  DatabaseConfig
	Redis     RedisConfig
	JWT       JWTConfig
	OAuth     OAuthConfig
	Storage   StorageConfig
	Firebase  FirebaseConfig
	Geocoding GeocodingConfig
	RateLimit RateLimitConfig
	Email     EmailConfig
	CORS      CORSConfig
	Monitoring MonitoringConfig
	Crypto    CryptoConfig
	Backup    BackupConfig
}

// BackupConfig holds database backup automation settings. The passphrase is
// used to symmetrically encrypt every pg_dump via gpg before the file
// touches disk, so a leak of the local volume or the MinIO bucket alone is
// not sufficient to recover plaintext data. Without a passphrase the
// backup job logs an error and refuses to run.
type BackupConfig struct {
	Enabled    bool
	LocalDir   string
	Bucket     string
	Passphrase string
}

// CryptoConfig holds at-rest encryption configuration. MFASecretKey is a
// 32-byte key encoded as 64 hex chars (generate with `openssl rand -hex 32`).
// When empty, MFA secrets fall back to plaintext storage — functional but
// non-compliant; flag warns at boot.
type CryptoConfig struct {
	MFASecretKey string
}

// ServerConfig holds server configuration
type ServerConfig struct {
	Port     string
	Host     string
	Env      string
	LogLevel string
	// AdminCookieDomain scopes the admin SPA's HttpOnly auth cookies. Empty
	// means host-only (cookie limited to the exact host that issued it),
	// which is correct for single-domain admin deployments. Set to e.g.
	// ".hamsaya.af" only when the admin panel and API live on different
	// subdomains under a shared parent.
	AdminCookieDomain string
}

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	Host            string
	Port            string
	Name            string
	User            string
	Password        string
	SSLMode         string
	MaxConns        int32
	MinConns        int32
	MaxConnLifetime time.Duration
	MaxConnIdleTime time.Duration

	// Optional read replica. When ReplicaHost is non-empty, repositories
	// can route hot reads (feed, search, profile lookups) to the replica
	// to keep the primary's CPU + IO headroom for writes. Replica
	// password / port / db default to the primary's when empty.
	ReplicaHost     string
	ReplicaPort     string
	ReplicaUser     string
	ReplicaPassword string
}

// RedisConfig holds Redis configuration
type RedisConfig struct {
	Host     string
	Port     string
	Password string
	DB       int
}

// JWTConfig holds JWT configuration
type JWTConfig struct {
	Secret               string
	AccessTokenDuration  time.Duration
	RefreshTokenDuration time.Duration
	// RefreshGrace is how long a rotated refresh token is still honored. Within
	// this window, presenting the old token returns the cached new pair instead
	// of revoking the session. Outside it, presenting a rotated token triggers
	// reuse detection and revokes the whole session family. 0 disables grace.
	RefreshGrace time.Duration
	// DeviceCredentialDuration sets the TTL for /auth/device/login secrets.
	// 0 means non-expiring (until explicit revoke).
	DeviceCredentialDuration time.Duration
}

// OAuthConfig holds OAuth provider configurations
type OAuthConfig struct {
	Google   GoogleOAuthConfig
	Apple    AppleOAuthConfig
	Facebook FacebookOAuthConfig
}

// GoogleOAuthConfig holds Google OAuth configuration
type GoogleOAuthConfig struct {
	ClientID     string
	ClientSecret string
}

// AppleOAuthConfig holds Apple OAuth configuration
type AppleOAuthConfig struct {
	ClientID   string
	TeamID     string
	KeyID      string
	PrivateKey string
}

// FacebookOAuthConfig holds Facebook OAuth configuration
type FacebookOAuthConfig struct {
	AppID     string
	AppSecret string
}

// StorageConfig holds object storage configuration
type StorageConfig struct {
	Endpoint   string
	AccessKey  string
	SecretKey  string
	BucketName string
	UseSSL     bool
	Region     string
	CDNURL     string
}

// FirebaseConfig holds Firebase configuration
type FirebaseConfig struct {
	ProjectID       string
	PrivateKey      string
	ClientEmail     string
	CredentialsPath string
}

// GeocodingConfig holds geocoding service configuration
type GeocodingConfig struct {
	APIKey   string
	Provider string
}

// RateLimitConfig holds rate limiting configuration
type RateLimitConfig struct {
	RequestsPerHour int
	AuthAttempts    int
	AuthWindow      time.Duration
}

// EmailConfig holds email configuration (SMTP and/or Resend)
type EmailConfig struct {
	SMTPHost          string
	SMTPPort          string
	User              string
	Password          string
	From              string
	ResendAPIKey      string // When set, send via Resend API instead of SMTP
	EmailVerifyBaseURL string // Base URL for verification link (e.g. https://hamsaya.com or app deep link)
}

// CORSConfig holds CORS configuration
type CORSConfig struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	AllowCredentials bool
}

// MonitoringConfig holds monitoring and observability configuration
type MonitoringConfig struct {
	SentryDSN            string
	PrometheusEnabled    bool
	ObservabilityEnabled bool
	OTLPEndpoint         string
	TraceSamplingRate    float64
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	viper.SetConfigFile(".env")
	viper.AutomaticEnv()

	// Ignore error if .env file doesn't exist
	_ = viper.ReadInConfig()

	cfg := &Config{
		Server: ServerConfig{
			Port:              viper.GetString("SERVER_PORT"),
			Host:              viper.GetString("SERVER_HOST"),
			Env:               viper.GetString("ENV"),
			LogLevel:          viper.GetString("LOG_LEVEL"),
			AdminCookieDomain: viper.GetString("ADMIN_COOKIE_DOMAIN"),
		},
		Database: DatabaseConfig{
			Host:            viper.GetString("DB_HOST"),
			Port:            viper.GetString("DB_PORT"),
			Name:            viper.GetString("DB_NAME"),
			User:            viper.GetString("DB_USER"),
			Password:        viper.GetString("DB_PASSWORD"),
			SSLMode:         viper.GetString("DB_SSL_MODE"),
			MaxConns:        getInt32("DB_MAX_CONNS"),
			MinConns:        getInt32("DB_MIN_CONNS"),
			MaxConnLifetime: viper.GetDuration("DB_MAX_CONN_LIFETIME"),
			MaxConnIdleTime: viper.GetDuration("DB_MAX_CONN_IDLE_TIME"),
			ReplicaHost:     viper.GetString("DB_REPLICA_HOST"),
			ReplicaPort:     viper.GetString("DB_REPLICA_PORT"),
			ReplicaUser:     viper.GetString("DB_REPLICA_USER"),
			ReplicaPassword: viper.GetString("DB_REPLICA_PASSWORD"),
		},
		Redis: RedisConfig{
			Host:     viper.GetString("REDIS_HOST"),
			Port:     viper.GetString("REDIS_PORT"),
			Password: viper.GetString("REDIS_PASSWORD"),
			DB:       viper.GetInt("REDIS_DB"),
		},
		JWT: JWTConfig{
			Secret:                   viper.GetString("JWT_SECRET"),
			AccessTokenDuration:      viper.GetDuration("JWT_ACCESS_TOKEN_DURATION"),
			RefreshTokenDuration:     viper.GetDuration("JWT_REFRESH_TOKEN_DURATION"),
			RefreshGrace:             viper.GetDuration("JWT_REFRESH_GRACE"),
			DeviceCredentialDuration: viper.GetDuration("DEVICE_CREDENTIAL_DURATION"),
		},
		OAuth: OAuthConfig{
			Google: GoogleOAuthConfig{
				ClientID:     viper.GetString("GOOGLE_CLIENT_ID"),
				ClientSecret: viper.GetString("GOOGLE_CLIENT_SECRET"),
			},
			Apple: AppleOAuthConfig{
				ClientID:   viper.GetString("APPLE_CLIENT_ID"),
				TeamID:     viper.GetString("APPLE_TEAM_ID"),
				KeyID:      viper.GetString("APPLE_KEY_ID"),
				PrivateKey: viper.GetString("APPLE_PRIVATE_KEY"),
			},
			Facebook: FacebookOAuthConfig{
				AppID:     viper.GetString("FACEBOOK_APP_ID"),
				AppSecret: viper.GetString("FACEBOOK_APP_SECRET"),
			},
		},
		Storage: StorageConfig{
			Endpoint:   viper.GetString("STORAGE_ENDPOINT"),
			AccessKey:  viper.GetString("STORAGE_ACCESS_KEY"),
			SecretKey:  viper.GetString("STORAGE_SECRET_KEY"),
			BucketName: viper.GetString("STORAGE_BUCKET_NAME"),
			UseSSL:     viper.GetBool("STORAGE_USE_SSL"),
			Region:     viper.GetString("STORAGE_REGION"),
			CDNURL:     viper.GetString("CDN_URL"),
		},
		Firebase: FirebaseConfig{
			ProjectID:       viper.GetString("FIREBASE_PROJECT_ID"),
			PrivateKey:      viper.GetString("FIREBASE_PRIVATE_KEY"),
			ClientEmail:     viper.GetString("FIREBASE_CLIENT_EMAIL"),
			CredentialsPath: viper.GetString("FIREBASE_CREDENTIALS_PATH"),
		},
		Geocoding: GeocodingConfig{
			APIKey:   viper.GetString("GEOCODING_API_KEY"),
			Provider: viper.GetString("GEOCODING_PROVIDER"),
		},
		RateLimit: RateLimitConfig{
			RequestsPerHour: viper.GetInt("RATE_LIMIT_REQUESTS_PER_HOUR"),
			AuthAttempts:    viper.GetInt("RATE_LIMIT_AUTH_ATTEMPTS"),
			AuthWindow:      viper.GetDuration("RATE_LIMIT_AUTH_WINDOW"),
		},
		Email: EmailConfig{
			SMTPHost:           viper.GetString("SMTP_HOST"),
			SMTPPort:           viper.GetString("SMTP_PORT"),
			User:               viper.GetString("SMTP_USER"),
			Password:           viper.GetString("SMTP_PASSWORD"),
			From:               viper.GetString("EMAIL_FROM"),
			ResendAPIKey:       viper.GetString("RESEND_API_KEY"),
			EmailVerifyBaseURL: viper.GetString("EMAIL_VERIFY_BASE_URL"),
		},
		CORS: CORSConfig{
			AllowedOrigins:   parseStringSlice(viper.GetString("CORS_ALLOWED_ORIGINS")),
			AllowedMethods:   parseStringSlice(viper.GetString("CORS_ALLOWED_METHODS")),
			AllowedHeaders:   parseStringSlice(viper.GetString("CORS_ALLOWED_HEADERS")),
			AllowCredentials: viper.GetBool("CORS_ALLOW_CREDENTIALS"),
		},
		Monitoring: MonitoringConfig{
			SentryDSN:            viper.GetString("SENTRY_DSN"),
			PrometheusEnabled:    viper.GetBool("PROMETHEUS_ENABLED"),
			ObservabilityEnabled: viper.GetBool("OBSERVABILITY_ENABLED"),
			OTLPEndpoint:         viper.GetString("OTLP_ENDPOINT"),
			TraceSamplingRate:    viper.GetFloat64("TRACE_SAMPLING_RATE"),
		},
		Crypto: CryptoConfig{
			MFASecretKey: viper.GetString("MFA_SECRET_ENCRYPTION_KEY"),
		},
		Backup: BackupConfig{
			Enabled:    viper.GetBool("BACKUP_ENABLED"),
			LocalDir:   viper.GetString("BACKUP_LOCAL_DIR"),
			Bucket:     viper.GetString("BACKUP_BUCKET"),
			Passphrase: viper.GetString("BACKUP_PASSPHRASE"),
		},
	}

	// Default observability settings
	if cfg.Monitoring.TraceSamplingRate == 0 {
		// Default to 10% sampling in production, 100% in development
		if cfg.Server.Env == "production" {
			cfg.Monitoring.TraceSamplingRate = 0.1
		} else {
			cfg.Monitoring.TraceSamplingRate = 1.0
		}
	}

	// pgxpool defaults when env vars are unset. Without these, pgxpool falls
	// back to its built-in default of 4 max connections which throttles even
	// modest production loads (Postgres-side context switches are cheap
	// compared to acquire-wait time on the pool).
	//
	// Production targets a single web instance handling ~200 RPS:
	//   * MaxConns 25 leaves headroom for pgbouncer/replicas if added later
	//   * MinConns 5 keeps a warm pool to absorb traffic spikes
	//   * MaxConnLifetime 1h forces periodic reconnects so long-lived
	//     connections pick up server-side config / TLS rotation.
	//   * MaxConnIdleTime 30m frees up idle workers during quiet hours.
	if cfg.Database.MaxConns == 0 {
		if cfg.Server.Env == "production" {
			cfg.Database.MaxConns = 25
		} else {
			cfg.Database.MaxConns = 10
		}
	}
	if cfg.Database.MinConns == 0 {
		if cfg.Server.Env == "production" {
			cfg.Database.MinConns = 5
		} else {
			cfg.Database.MinConns = 2
		}
	}
	if cfg.Database.MaxConnLifetime == 0 {
		cfg.Database.MaxConnLifetime = time.Hour
	}
	if cfg.Database.MaxConnIdleTime == 0 {
		cfg.Database.MaxConnIdleTime = 30 * time.Minute
	}

	// Reject weak or default JWT secrets at startup to prevent accidental insecure deployments.
	const defaultJWTSecret = "your-super-secret-jwt-key-change-this-in-production"
	if cfg.JWT.Secret == "" || cfg.JWT.Secret == defaultJWTSecret || len(cfg.JWT.Secret) < 32 {
		return nil, fmt.Errorf(
			"JWT_SECRET must be set to a strong, unique secret of at least 32 characters " +
				"(current value is empty, the default placeholder, or too short)")
	}

	// Require MFA encryption key: non-empty and a valid 32-byte hex string (64 hex chars).
	// pkg/crypto.NewSecretCipher enforces the same shape; validating here fails fast at boot
	// instead of at first MFA operation.
	if cfg.Crypto.MFASecretKey == "" {
		return nil, fmt.Errorf(
			"MFA_SECRET_ENCRYPTION_KEY must be set (32-byte hex, 64 characters) — " +
				"generate with: openssl rand -hex 32")
	}
	if len(cfg.Crypto.MFASecretKey) != 64 {
		return nil, fmt.Errorf(
			"MFA_SECRET_ENCRYPTION_KEY must be 64 hex characters (32 bytes); got %d characters",
			len(cfg.Crypto.MFASecretKey))
	}

	// Reject default MinIO dev credential for object storage to prevent accidental
	// deployment with well-known keys.
	const defaultStorageSecretKey = "minioadmin"
	if cfg.Storage.SecretKey == "" || cfg.Storage.SecretKey == defaultStorageSecretKey {
		return nil, fmt.Errorf(
			"STORAGE_SECRET_KEY must be set to a non-default value " +
				"(current value is empty or the well-known MinIO default 'minioadmin')")
	}

	// Default CORS in development so admin panel (e.g. localhost:3001) works without .env
	if cfg.Server.Env == "development" {
		if len(cfg.CORS.AllowedOrigins) == 0 {
			cfg.CORS.AllowedOrigins = []string{
				"http://localhost:3000", "http://localhost:3001", "http://localhost:5173",
				"http://127.0.0.1:3000", "http://127.0.0.1:3001", "http://127.0.0.1:5173",
			}
		}
		if len(cfg.CORS.AllowedMethods) == 0 {
			cfg.CORS.AllowedMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"}
		}
		if len(cfg.CORS.AllowedHeaders) == 0 {
			cfg.CORS.AllowedHeaders = []string{"Content-Type", "Authorization", "Accept", "Origin", "User-Agent", "X-CSRF-Token", "X-Device-Info"}
		}
		if !cfg.CORS.AllowCredentials {
			// Admin SPA depends on credentialed cross-origin requests when
			// running on a separate dev port. Force it on in development so
			// HttpOnly cookies flow.
			cfg.CORS.AllowCredentials = true
		}
	}

	// Reject the unsafe combination of credentialed CORS with a wildcard
	// origin: browsers ignore the response, but the misconfiguration tends to
	// hide a real bug (someone meant to allowlist explicit origins).
	if cfg.CORS.AllowCredentials {
		for _, o := range cfg.CORS.AllowedOrigins {
			if strings.TrimSpace(o) == "*" {
				return nil, fmt.Errorf(
					"CORS_ALLOWED_ORIGINS cannot contain '*' when CORS_ALLOW_CREDENTIALS=true; " +
						"list explicit origins (e.g. https://admin.hamsaya.af)")
			}
		}
	}

	return cfg, nil
}

// GetDSN returns the PostgreSQL connection string
func (c *DatabaseConfig) GetDSN() string {
	return "postgres://" + c.User + ":" + c.Password + "@" + c.Host + ":" + c.Port + "/" + c.Name + "?sslmode=" + c.SSLMode
}

// GetReplicaDSN returns the read-replica connection string, or "" when no
// replica is configured. Falls back to the primary's port/user/password
// when the replica-specific overrides are empty.
func (c *DatabaseConfig) GetReplicaDSN() string {
	if c.ReplicaHost == "" {
		return ""
	}
	port := c.ReplicaPort
	if port == "" {
		port = c.Port
	}
	user := c.ReplicaUser
	if user == "" {
		user = c.User
	}
	password := c.ReplicaPassword
	if password == "" {
		password = c.Password
	}
	return "postgres://" + user + ":" + password + "@" + c.ReplicaHost + ":" + port + "/" + c.Name + "?sslmode=" + c.SSLMode
}

// GetRedisAddr returns Redis address
func (c *RedisConfig) GetAddr() string {
	return c.Host + ":" + c.Port
}
