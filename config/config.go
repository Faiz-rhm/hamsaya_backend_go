package config

import (
	"time"

	"github.com/spf13/viper"
)

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
}

// ServerConfig holds server configuration
type ServerConfig struct {
	Port     string
	Host     string
	Env      string
	LogLevel string
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

// EmailConfig holds email configuration
type EmailConfig struct {
	SMTPHost string
	SMTPPort string
	User     string
	Password string
	From     string
}

// CORSConfig holds CORS configuration
type CORSConfig struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	AllowCredentials bool
}

// MonitoringConfig holds monitoring configuration
type MonitoringConfig struct {
	SentryDSN        string
	PrometheusEnabled bool
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	viper.SetConfigFile(".env")
	viper.AutomaticEnv()

	// Ignore error if .env file doesn't exist
	_ = viper.ReadInConfig()

	cfg := &Config{
		Server: ServerConfig{
			Port:     viper.GetString("SERVER_PORT"),
			Host:     viper.GetString("SERVER_HOST"),
			Env:      viper.GetString("ENV"),
			LogLevel: viper.GetString("LOG_LEVEL"),
		},
		Database: DatabaseConfig{
			Host:            viper.GetString("DB_HOST"),
			Port:            viper.GetString("DB_PORT"),
			Name:            viper.GetString("DB_NAME"),
			User:            viper.GetString("DB_USER"),
			Password:        viper.GetString("DB_PASSWORD"),
			SSLMode:         viper.GetString("DB_SSL_MODE"),
			MaxConns:        int32(viper.GetInt("DB_MAX_CONNS")),
			MinConns:        int32(viper.GetInt("DB_MIN_CONNS")),
			MaxConnLifetime: viper.GetDuration("DB_MAX_CONN_LIFETIME"),
			MaxConnIdleTime: viper.GetDuration("DB_MAX_CONN_IDLE_TIME"),
		},
		Redis: RedisConfig{
			Host:     viper.GetString("REDIS_HOST"),
			Port:     viper.GetString("REDIS_PORT"),
			Password: viper.GetString("REDIS_PASSWORD"),
			DB:       viper.GetInt("REDIS_DB"),
		},
		JWT: JWTConfig{
			Secret:               viper.GetString("JWT_SECRET"),
			AccessTokenDuration:  viper.GetDuration("JWT_ACCESS_TOKEN_DURATION"),
			RefreshTokenDuration: viper.GetDuration("JWT_REFRESH_TOKEN_DURATION"),
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
			SMTPHost: viper.GetString("SMTP_HOST"),
			SMTPPort: viper.GetString("SMTP_PORT"),
			User:     viper.GetString("SMTP_USER"),
			Password: viper.GetString("SMTP_PASSWORD"),
			From:     viper.GetString("EMAIL_FROM"),
		},
		CORS: CORSConfig{
			AllowedOrigins:   viper.GetStringSlice("CORS_ALLOWED_ORIGINS"),
			AllowedMethods:   viper.GetStringSlice("CORS_ALLOWED_METHODS"),
			AllowedHeaders:   viper.GetStringSlice("CORS_ALLOWED_HEADERS"),
			AllowCredentials: viper.GetBool("CORS_ALLOW_CREDENTIALS"),
		},
		Monitoring: MonitoringConfig{
			SentryDSN:         viper.GetString("SENTRY_DSN"),
			PrometheusEnabled: viper.GetBool("PROMETHEUS_ENABLED"),
		},
	}

	return cfg, nil
}

// GetDSN returns the PostgreSQL connection string
func (c *DatabaseConfig) GetDSN() string {
	return "postgres://" + c.User + ":" + c.Password + "@" + c.Host + ":" + c.Port + "/" + c.Name + "?sslmode=" + c.SSLMode
}

// GetRedisAddr returns Redis address
func (c *RedisConfig) GetAddr() string {
	return c.Host + ":" + c.Port
}
