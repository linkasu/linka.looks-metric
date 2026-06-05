package config

import (
	"errors"
	"net"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	AppHost       string
	AppPort       string
	DatabasePath  string
	PublicBaseURL string
	Mail          MailConfig
}

type MailConfig struct {
	Host      string
	Port      int
	User      string
	Password  string
	FromEmail string
	FromName  string
	DryRun    bool
}

func Load() (Config, error) {
	cfg := Config{
		AppHost:       env("APP_HOST", "0.0.0.0"),
		AppPort:       env("APP_PORT", "30812"),
		DatabasePath:  normalizeDBPath(env("DATABASE_URL", env("DATABASE_PATH", "/data/metric.db"))),
		PublicBaseURL: env("PUBLIC_BASE_URL", "https://metric.linka.su"),
		Mail: MailConfig{
			Host:      env("SMTP_HOST", "smtp.yandex.ru"),
			Port:      envInt("SMTP_PORT", 465),
			User:      env("SMTP_USER", env("EMAIL", "")),
			Password:  env("SMTP_PASSWORD", env("EMAIL_PASSWORD", "")),
			FromEmail: env("SMTP_FROM_EMAIL", env("EMAIL", "apps@linka.su")),
			FromName:  env("SMTP_FROM_NAME", "LINKa"),
			DryRun:    envBool("MAIL_DRY_RUN", false),
		},
	}

	if cfg.DatabasePath == "" {
		return cfg, errors.New("DATABASE_URL or DATABASE_PATH is required")
	}
	if cfg.Mail.User == "" && !cfg.Mail.DryRun {
		return cfg, errors.New("SMTP_USER or EMAIL is required")
	}
	if cfg.Mail.Password == "" && !cfg.Mail.DryRun {
		return cfg, errors.New("SMTP_PASSWORD or EMAIL_PASSWORD is required")
	}
	return cfg, nil
}

func (c Config) Addr() string {
	return net.JoinHostPort(c.AppHost, c.AppPort)
}

func env(name, fallback string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	return value
}

func envInt(name string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envBool(name string, fallback bool) bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(name)))
	if value == "" {
		return fallback
	}
	return value == "1" || value == "true" || value == "yes"
}

func normalizeDBPath(value string) string {
	if strings.HasPrefix(value, "file:") {
		value = strings.TrimPrefix(value, "file:")
		if idx := strings.IndexByte(value, '?'); idx >= 0 {
			value = value[:idx]
		}
	}
	return value
}
