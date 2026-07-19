package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	AppEnv              string
	AppPort             string
	DBHost              string
	DBPort              string
	DBUser              string
	DBPassword          string
	DBName              string
	JWTSecret           string
	JWTExpiryHours      int
	WorkspaceRestricted  bool
	AllowRegistration    bool
	CORSOrigins          string
	R2AccountID          string
	R2AccessKey         string
	R2SecretKey         string
	R2BucketName        string
	R2PublicURL         string
	APIHost             string
	FrontURL			string
	SMTPHost            string
	SMTPPort            int
	SMTPUser            string
	SMTPPassword        string
	SMTPFrom            string
	SMTPFromName        string
	GoogleClientID      string
	GoogleClientSecret  string
	GoogleRedirectURL   string
	GitHubClientID      string
	GitHubClientSecret  string
	GitHubRedirectURL   string
	GitLabClientID      string
	GitLabClientSecret  string
	GitLabRedirectURL   string
	DBMaxIdleConns      int
	DBMaxOpenConns      int
	DBConnMaxLifetimeMinutes int
	LinkCheckIntervalHours int
	BrokenLinkThreshold int
}

var AppConfig *Config

func LoadConfig() {
	_ = godotenv.Load() // Load .env file if it exists

	AppConfig = &Config{
		AppEnv:              getEnv("APP_ENV", "development"),
		AppPort:             getEnv("APP_PORT", "8080"),
		DBHost:              getEnv("DB_HOST", "127.0.0.1"),
		DBPort:             getEnv("DB_PORT", "3306"),
		DBUser:              getEnv("DB_USER", "root"),
		DBPassword:          getEnv("DB_PASSWORD", ""),
		DBName:              getEnv("DB_NAME", "kontent"),
		JWTSecret:           getEnv("JWT_SECRET", "super-secret-key"),
		JWTExpiryHours:      getEnvInt("JWT_EXPIRY_HOURS", 24),
		WorkspaceRestricted:  getEnvBool("WORKSPACE_RESTRICTED", false),
		AllowRegistration:    getEnvBool("ALLOW_REGISTRATION", true),
		CORSOrigins:          getEnv("CORS_ORIGINS", "http://localhost:3000,http://127.0.0.1:3000"),
		R2AccountID:          getEnv("R2_ACCOUNT_ID", ""),
		R2AccessKey:         getEnv("R2_ACCESS_KEY_ID", ""),
		R2SecretKey:         getEnv("R2_SECRET_ACCESS_KEY", ""),
		R2BucketName:        getEnv("R2_BUCKET_NAME", ""),
		R2PublicURL:         getEnv("R2_PUBLIC_URL", ""),
		APIHost:             getEnv("API_HOST", "api.tulis.org"),
		SMTPHost:            getEnv("SMTP_HOST", "localhost"),
		SMTPPort:            getEnvInt("SMTP_PORT", 1025),
		SMTPUser:            getEnv("SMTP_USER", ""),
		SMTPPassword:        getEnv("SMTP_PASSWORD", ""),
		SMTPFrom:            getEnv("SMTP_FROM", "hello@tulis.org"),
		SMTPFromName:        getEnv("SMTP_FROM_NAME", "Tulis CMS"),
		GoogleClientID:      getEnv("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret:  getEnv("GOOGLE_CLIENT_SECRET", ""),
		GoogleRedirectURL:   getEnv("GOOGLE_REDIRECT_URL", "http://localhost:8080/api/auth/google/callback"),
		GitHubClientID:      getEnv("GITHUB_CLIENT_ID", ""),
		GitHubClientSecret:  getEnv("GITHUB_CLIENT_SECRET", ""),
		GitHubRedirectURL:   getEnv("GITHUB_REDIRECT_URL", "http://localhost:8080/api/auth/github/callback"),
		GitLabClientID:      getEnv("GITLAB_CLIENT_ID", ""),
		GitLabClientSecret:  getEnv("GITLAB_CLIENT_SECRET", ""),
		GitLabRedirectURL:   getEnv("GITLAB_REDIRECT_URL", "http://localhost:8080/api/auth/gitlab/callback"),
		DBMaxIdleConns:      getEnvInt("DB_MAX_IDLE_CONNS", 10),
		DBMaxOpenConns:      getEnvInt("DB_MAX_OPEN_CONNS", 100),
		DBConnMaxLifetimeMinutes: getEnvInt("DB_CONN_MAX_LIFETIME_MINS", 60),
		FrontURL : getEnv("FRONTEND_URL","http://localhost:3000"),
		LinkCheckIntervalHours: getEnvInt("LINK_CHECK_INTERVAL_HOURS", 24),
		BrokenLinkThreshold: getEnvInt("BROKEN_LINK_THRESHOLD", 5),
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if value, ok := os.LookupEnv(key); ok {
		if val, err := strconv.Atoi(value); err == nil {
			return val
		}
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if value, ok := os.LookupEnv(key); ok {
		return value == "true" || value == "1" || value == "yes"
	}
	return fallback
}
