package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Server   ServerConfig
	WhatsApp WhatsAppConfig
	Auth     AuthConfig
	Database DatabaseConfig
}

type ServerConfig struct {
	Port            string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
	MaxUploadSize   int64
}

type WhatsAppConfig struct {
	SessionKey     string
	DefaultCountry string
	QRCodeGenerate bool
	ReconnectDelay time.Duration
}

type AuthConfig struct {
	APIToken   string
	SessionKey string
}

type DatabaseConfig struct {
	Driver string
	DSN    string
}

func Load() (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			Port:            getEnv("SERVER_PORT", "8080"),
			ReadTimeout:     getDurationEnv("SERVER_READ_TIMEOUT", 15*time.Second),
			WriteTimeout:    getDurationEnv("SERVER_WRITE_TIMEOUT", 15*time.Second),
			IdleTimeout:     getDurationEnv("SERVER_IDLE_TIMEOUT", 60*time.Second),
			ShutdownTimeout: getDurationEnv("SERVER_SHUTDOWN_TIMEOUT", 10*time.Second),
			MaxUploadSize:   getInt64Env("MAX_UPLOAD_SIZE", 50<<20), // 50MB
		},
		WhatsApp: WhatsAppConfig{
			SessionKey:     getEnv("WHATSAPP_SESSION_KEY", "default-session"),
			DefaultCountry: getEnv("WHATSAPP_DEFAULT_COUNTRY", "55"),
			QRCodeGenerate: getBoolEnv("WHATSAPP_QR_GENERATE", true),
			ReconnectDelay: getDurationEnv("WHATSAPP_RECONNECT_DELAY", 5*time.Second),
		},
		Auth: AuthConfig{
			APIToken:   getEnv("API_TOKEN", ""),
			SessionKey: getEnv("SESSION_KEY", ""),
		},
		Database: DatabaseConfig{
			Driver: getEnv("DB_DRIVER", "sqlite3"),
			DSN:    getEnv("DB_DSN", "file:whatsapp.db?_foreign_keys=on"),
		},
	}

	if cfg.Auth.APIToken == "" {
		return nil, fmt.Errorf("API_TOKEN is required")
	}
	if cfg.Auth.SessionKey == "" {
		return nil, fmt.Errorf("SESSION_KEY is required")
	}

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getInt64Env(key string, defaultValue int64) int64 {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.ParseInt(value, 10, 64); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getBoolEnv(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

func getDurationEnv(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}
