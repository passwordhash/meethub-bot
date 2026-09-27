package channels

import "github.com/passwordhash/cascade-hub-bot/internal/channel/models"

func entityToModel(entity channelEntity) models.Channel {
	return models.Channel{ChannelID: entity.ChannelID, GroupID: entity.GroupID}
}

func scheduleEntityToModel(entity scheduleEntity) models.Schedule {
	schedule := models.Schedule{ChannelID: entity.ChannelID, Override: entity.Override, CheckedAt: entity.CheckedAt}
	for _, day := range entity.Weekdays {
		schedule.Weekdays = append(schedule.Weekdays, int(day))
	}
	if entity.Hour != nil {
		schedule.Hour = int(*entity.Hour)
	}
	if entity.Minute != nil {
		schedule.Minute = int(*entity.Minute)
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
