package config

import (
	"fmt"
	"net/url"

	"github.com/caarlos0/env/v11"
)

type Config struct {
	Telegram   TelegramConfig
	Postgres   PostgresConfig
	MeetingURL string `env:"MEETING_URL,required"`
}

type TelegramConfig struct {
	Token   string `env:"TELEGRAM_BOT_TOKEN,required"`
	OwnerID int64  `env:"TELEGRAM_OWNER_ID,required"`
}

type PostgresConfig struct {
	Host     string `env:"POSTGRES_HOST,required"`
	Port     uint16 `env:"POSTGRES_PORT,required"`
	Database string `env:"POSTGRES_DATABASE,required"`
	User     string `env:"POSTGRES_USER,required"`
	Password string `env:"POSTGRES_PASSWORD,required"`
	SSLMode  string `env:"POSTGRES_SSLMODE,required"`
}

func validateMeetingURL(value string) error {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return fmt.Errorf("MEETING_URL must be a valid HTTPS URL")
	}
	return nil
}

func Load() (Config, error) {
	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return Config{}, fmt.Errorf("parse configuration: %w", err)
	}
	if cfg.Telegram.OwnerID <= 0 {
		return Config{}, fmt.Errorf("TELEGRAM_OWNER_ID must be positive")
	}
	if err := validateMeetingURL(cfg.MeetingURL); err != nil {
		return Config{}, err
	}
	if cfg.Postgres.Port == 0 {
		return Config{}, fmt.Errorf("POSTGRES_PORT must be positive")
	}
	return cfg, nil
}
