package models

import (
	"errors"
	"regexp"
	"strings"
	"unicode/utf8"
)

var ErrInvalidParticipant = errors.New("invalid participant")

// MaxGitLabUsernameLength keeps the removal button within Telegram's 64-byte callback data.
const MaxGitLabUsernameLength = 39
const MaxTelegramLabelLength = 64

var gitLabUsernamePattern = regexp.MustCompile(`^[a-z0-9_.-]+$`)

type Participant struct {
	ChannelID      int64
	GitLabUsername string
	TelegramLabel  string
}

type GetParticipantsFilters struct {
	ChannelID *int64
}

func ValidateGitLabUsername(username string) error {
	if len(username) > MaxGitLabUsernameLength || !gitLabUsernamePattern.MatchString(username) {
		return ErrInvalidParticipant
	}
	return nil
}

// ParseParticipant reads "<GitLab username> <Telegram label>", e.g. "yarik @yarik".
func ParseParticipant(text string) (string, string, error) {
	username, label, found := strings.Cut(strings.TrimSpace(text), " ")
	username = strings.ToLower(strings.TrimPrefix(username, "@"))
	label = strings.TrimSpace(label)
	if !found || ValidateGitLabUsername(username) != nil || label == "" || utf8.RuneCountInString(label) > MaxTelegramLabelLength || strings.ContainsAny(label, "\n\r") {
		return "", "", ErrInvalidParticipant
	}
	return username, label, nil
}
