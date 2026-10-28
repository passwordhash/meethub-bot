package config

import (
	"testing"
	"time"
)

func TestGitLabConfigValidate(t *testing.T) {
	valid := GitLabConfig{URL: "https://gitlab.example.com", Token: "token", Group: "team", PollInterval: 90 * time.Second}
	tests := []struct {
		name    string
		mutate  func(*GitLabConfig)
		enabled bool
		wantErr bool
	}{
		{name: "disabled", mutate: func(c *GitLabConfig) { *c = GitLabConfig{PollInterval: 90 * time.Second} }},
		{name: "complete", mutate: func(*GitLabConfig) {}, enabled: true},
		{name: "plain HTTP", mutate: func(c *GitLabConfig) { c.URL = "http://gitlab.local" }, enabled: true},
		{name: "missing token", mutate: func(c *GitLabConfig) { c.Token = "" }, enabled: true, wantErr: true},
		{name: "missing group", mutate: func(c *GitLabConfig) { c.Group = "" }, enabled: true, wantErr: true},
		{name: "invalid URL", mutate: func(c *GitLabConfig) { c.URL = "gitlab.example.com" }, enabled: true, wantErr: true},
		{name: "too frequent polling", mutate: func(c *GitLabConfig) { c.PollInterval = 10 * time.Second }, enabled: true, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := valid
			tt.mutate(&cfg)
			if cfg.Enabled() != tt.enabled {
				t.Fatalf("Enabled() = %t; want %t", cfg.Enabled(), tt.enabled)
			}
			if err := cfg.validate(); (err != nil) != tt.wantErr {
				t.Fatalf("validate() = %v; wantErr %t", err, tt.wantErr)
			}
		})
	}
}
