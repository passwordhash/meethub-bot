package config

import (
	"fmt"
	"net/url"
	"time"

	"github.com/caarlos0/env/v11"
)

type Config struct {
	Telegram   TelegramConfig
	Postgres   PostgresConfig
	GitLab     GitLabConfig
	MeetingURL string `env:"MEETING_URL,required"`
}

// GitLabConfig is optional: without it the MR board is off.
type GitLabConfig struct {
	URL          string        `env:"GITLAB_URL"`
	Token        string        `env:"GITLAB_TOKEN"`
	Group        string        `env:"GITLAB_GROUP"`
	PollInterval time.Duration `env:"GITLAB_POLL_INTERVAL" envDefault:"90s"`
}

const MinGitLabPollInterval = 30 * time.Second

func (c GitLabConfig) Enabled() bool {
	return c.URL != "" || c.Token != "" || c.Group != ""
}

func (c GitLabConfig) validate() error {
	if !c.Enabled() {
		return nil
	}
	if c.URL == "" || c.Token == "" || c.Group == "" {
		return fmt.Errorf("GITLAB_URL, GITLAB_TOKEN and GITLAB_GROUP must be set together")
	}
	parsed, err := url.Parse(c.URL)
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" {
		return fmt.Errorf("GITLAB_URL must be a valid HTTP(S) URL")
	}
	if c.PollInterval < MinGitLabPollInterval {
		return fmt.Errorf("GITLAB_POLL_INTERVAL must be at least %s", MinGitLabPollInterval)
	}
	return nil
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
	if err := cfg.GitLab.validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}
