package models

import (
	"testing"
	"time"
)

func TestParseStartsAt(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		want    time.Duration
		wantErr bool
	}{
		{name: "midnight", text: "00:00", want: 0},
		{name: "arbitrary minute", text: "12:37", want: 12*time.Hour + 37*time.Minute},
		{name: "last minute of day", text: "23:59", want: 23*time.Hour + 59*time.Minute},
		{name: "single digit hour rejected", text: "9:30", wantErr: true},
		{name: "hour out of range", text: "24:00", wantErr: true},
		{name: "minute out of range", text: "10:60", wantErr: true},
		{name: "dot separator rejected", text: "10.30", wantErr: true},
		{name: "seconds rejected", text: "10:30:00", wantErr: true},
		{name: "empty", text: "", wantErr: true},
		{name: "text", text: "утро", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseStartsAt(tt.text)
			if (err != nil) != tt.wantErr || got != tt.want {
				t.Fatalf("ParseStartsAt(%q) = %v, %v", tt.text, got, err)
			}
		})
	}
}

func TestFormatStartsAt(t *testing.T) {
	if got := FormatStartsAt(9*time.Hour + 5*time.Minute); got != "09:05" {
		t.Fatalf("FormatStartsAt() = %q", got)
	}
}

func TestValidatePostOffset(t *testing.T) {
	tests := []struct {
		name    string
		minutes int
		wantErr bool
	}{
		{name: "zero", minutes: 0},
		{name: "maximum", minutes: 1439},
		{name: "negative", minutes: -1, wantErr: true},
		{name: "full day", minutes: 1440, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidatePostOffset(tt.minutes); (err != nil) != tt.wantErr {
				t.Fatalf("ValidatePostOffset(%d) = %v", tt.minutes, err)
			}
		})
	}
}

func TestNextDateOf(t *testing.T) {
	tests := []struct {
		name    string
		now     time.Time
		weekday time.Weekday
		want    time.Time
	}{
		{name: "same weekday is today", now: time.Date(2026, 10, 12, 23, 0, 0, 0, Moscow), weekday: time.Monday, want: time.Date(2026, 10, 12, 0, 0, 0, 0, Moscow)},
		{name: "later in week", now: time.Date(2026, 10, 12, 9, 0, 0, 0, Moscow), weekday: time.Friday, want: time.Date(2026, 10, 16, 0, 0, 0, 0, Moscow)},
		{name: "earlier weekday wraps to next week", now: time.Date(2026, 10, 14, 9, 0, 0, 0, Moscow), weekday: time.Monday, want: time.Date(2026, 10, 19, 0, 0, 0, 0, Moscow)},
		{name: "Sunday after Monday", now: time.Date(2026, 10, 12, 9, 0, 0, 0, Moscow), weekday: time.Sunday, want: time.Date(2026, 10, 18, 0, 0, 0, 0, Moscow)},
		{name: "UTC evening is next Moscow day", now: time.Date(2026, 10, 12, 21, 30, 0, 0, time.UTC), weekday: time.Tuesday, want: time.Date(2026, 10, 13, 0, 0, 0, 0, Moscow)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NextDateOf(tt.now, tt.weekday); !got.Equal(tt.want) {
				t.Fatalf("NextDateOf() = %v; want %v", got, tt.want)
			}
		})
	}
}
