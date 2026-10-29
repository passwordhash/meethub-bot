package channels

import (
	"time"

	"github.com/passwordhash/cascade-hub-bot/internal/channel/models"
)

func entityToModel(entity channelEntity) models.Channel {
	return models.Channel{ChannelID: entity.ChannelID, GroupID: entity.GroupID, BoardView: models.BoardView(entity.BoardView)}
}

func scheduleEntitiesToModels(entities []scheduleEntity, dailyTimes []dailyTimeEntity) []models.Schedule {
	schedules := make([]models.Schedule, 0, len(entities))
	indexes := make(map[int64]int, len(entities))
	for _, entity := range entities {
		indexes[entity.ChannelID] = len(schedules)
		schedules = append(schedules, models.Schedule{
			ChannelID:         entity.ChannelID,
			DailyTimes:        make(map[time.Weekday]time.Duration),
			PostOffsetMinutes: entity.PostOffsetMinutes,
			LeadEnabled:       entity.LeadEnabled,
			ResetPending:      entity.ResetPending,
			CheckedAt:         entity.CheckedAt,
			BoardView:         models.BoardView(entity.BoardView),
		})
	}
	for _, entity := range dailyTimes {
		if index, found := indexes[entity.ChannelID]; found {
			schedules[index].DailyTimes[time.Weekday(entity.Weekday)] = time.Duration(entity.StartsAtMinutes) * time.Minute
		}
	}
	return schedules
}

func ignoredUserEntityToModel(entity ignoredUserEntity) models.IgnoredUser {
	return models.IgnoredUser{ChannelID: entity.ChannelID, UserID: entity.UserID}
}

func participantEntityToModel(entity participantEntity) models.Participant {
	return models.Participant{ChannelID: entity.ChannelID, GitLabUsername: entity.GitLabUsername, TelegramLabel: entity.TelegramLabel}
}
