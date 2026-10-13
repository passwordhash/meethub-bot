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

type Channel struct {
	ChannelID int64
	GroupID   int64
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
}

type GetSchedulesFilters struct {
	ChannelID *int64
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
