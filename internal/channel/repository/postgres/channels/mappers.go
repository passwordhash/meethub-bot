package channels

import "github.com/passwordhash/cascade-hub-bot/internal/channel/models"

func entityToModel(entity channelEntity) models.Channel {
	return models.Channel{ChannelID: entity.ChannelID, GroupID: entity.GroupID}
}

func scheduleEntityToModel(entity scheduleEntity) models.Schedule {
	schedule := models.Schedule{ChannelID: entity.ChannelID, LeadEnabled: entity.LeadEnabled, Override: entity.Override, CheckedAt: entity.CheckedAt}
	if entity.Cron != nil {
		schedule.Cron = *entity.Cron
	}
	return schedule
}

func scheduleEntitiesToModels(entities []scheduleEntity) []models.Schedule {
	schedules := make([]models.Schedule, 0, len(entities))
	for _, entity := range entities {
		schedules = append(schedules, scheduleEntityToModel(entity))
	}
	return schedules
}
