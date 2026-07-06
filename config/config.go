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
	R2AccountID          string
	R2AccessKey         string
	R2SecretKey         string
	R2BucketName        string
	R2PublicURL         string
}

var AppConfig *Config

func LoadConfig() {
	_ = godotenv.Load() // Load .env file if it exists

	AppConfig = &Config{
		AppEnv:              getEnv("APP_ENV", "development"),
		AppPort:             getEnv("APP_PORT", "8080"),
		DBHost:              getEnv("DB_HOST", "127.0.0.1"),
		DBPort:              getEnv("DB_PORT", "3306"),
		DBUser:              getEnv("DB_USER", "root"),
		DBPassword:          getEnv("DB_PASSWORD", ""),
		DBName:              getEnv("DB_NAME", "kontent"),
		JWTSecret:           getEnv("JWT_SECRET", "super-secret-key"),
		JWTExpiryHours:      getEnvInt("JWT_EXPIRY_HOURS", 24),
		WorkspaceRestricted:  getEnvBool("WORKSPACE_RESTRICTED", false),
		AllowRegistration:    getEnvBool("ALLOW_REGISTRATION", true),
		R2AccountID:          getEnv("R2_ACCOUNT_ID", ""),
		R2AccessKey:         getEnv("R2_ACCESS_KEY_ID", ""),
		R2SecretKey:         getEnv("R2_SECRET_ACCESS_KEY", ""),
		R2BucketName:        getEnv("R2_BUCKET_NAME", ""),
		R2PublicURL:         getEnv("R2_PUBLIC_URL", ""),
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
