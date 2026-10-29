package models

import (
	"errors"
	"time"
)

var ErrInvalidChannelID = errors.New("invalid channel id")
var ErrAlreadyConnected = errors.New("channel is already connected to another group")
var ErrGroupAlreadyConnected = errors.New("group is already connected to another channel")
var ErrUnverifiablePermissions = errors.New("channel or group permissions could not be verified")
var ErrTelegramRejected = errors.New("telegram rejected the request")

// ErrBoardMessageMissing permits recreating a board, unlike permission or rate-limit failures.
var ErrBoardMessageMissing = errors.New("telegram board message is missing")
var ErrInvalidUserID = errors.New("invalid user id")

type Channel struct {
	ChannelID int64
	GroupID   int64
	BoardView BoardView
}

type GetChannelsFilters struct {
	ChannelID *int64
	GroupID   *int64
}

type Schedule struct {
	ChannelID         int64
	DailyTimes        map[time.Weekday]time.Duration
	PostOffsetMinutes int
	LeadEnabled       bool
	ResetPending      bool
	CheckedAt         *time.Time
	BoardView         BoardView
}

// BoardView picks how the MR board groups its rows.
type BoardView int16

const (
	BoardViewBalls    BoardView = 1
	BoardViewReviewer BoardView = 2
	BoardViewProject  BoardView = 3
)

// Label names the view on the owner menu button; an unset view reads as the default.
func (v BoardView) Label() string {
	switch v {
	case BoardViewReviewer:
		return "по ревьюверу"
	case BoardViewProject:
		return "по проекту"
	default:
		return "по мячам"
	}
}

// Next cycles through the views for the owner menu button.
func (v BoardView) Next() BoardView {
	switch v {
	case BoardViewBalls:
		return BoardViewReviewer
	case BoardViewReviewer:
		return BoardViewProject
	default:
		return BoardViewBalls
	}
}

type GetSchedulesFilters struct {
	ChannelID *int64
}

type IgnoredUser struct {
	ChannelID int64
	UserID    int64
}

type GetIgnoredUsersFilters struct {
	ChannelID *int64
	UserID    *int64
}

type Permissions struct {
	ChannelType       string
	LinkedGroupID     int64
	OwnerIsAdmin      bool
	BotIsAdmin        bool
	BotCanPost        bool
	GroupType         string
	GroupLinkedChatID int64
	BotInGroup        bool
	BotCanSend        bool
}

func ValidateChannelID(id int64) error {
	if id >= 0 {
		return ErrInvalidChannelID
	}
	return nil
}

func ValidateUserID(id int64) error {
	if id <= 0 {
		return ErrInvalidUserID
	}
	return nil
}

func (p Permissions) Validate(channelID int64) error {
	if p.ChannelType != "channel" || p.LinkedGroupID == 0 || p.LinkedGroupID == channelID || !p.OwnerIsAdmin || !p.BotIsAdmin || !p.BotCanPost {
		return ErrUnverifiablePermissions
	}
	if (p.GroupType != "group" && p.GroupType != "supergroup") || !p.BotInGroup || !p.BotCanSend {
		return ErrUnverifiablePermissions
	}
	if p.GroupLinkedChatID != 0 && p.GroupLinkedChatID != channelID {
		return ErrUnverifiablePermissions
	}
	return nil
}
