package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	AppEnv         string
	AppPort        string
	DatabaseURL    string
	DBDriver       string // sqlite, postgres, mysql
	JWTSecret      string
	JWTExpiryHours int
}

var AppConfig *Config

func LoadConfig() {
	_ = godotenv.Load() // Load .env file if it exists

	AppConfig = &Config{
		AppEnv:         getEnv("APP_ENV", "development"),
		AppPort:        getEnv("APP_PORT", "8080"),
		DatabaseURL:    getEnv("DATABASE_URL", "kontent.db"),
		DBDriver:       getEnv("DB_DRIVER", "sqlite"),
		JWTSecret:      getEnv("JWT_SECRET", "super-secret-key"),
		JWTExpiryHours: getEnvInt("JWT_EXPIRY_HOURS", 24),
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
