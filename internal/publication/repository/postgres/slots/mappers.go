package slots

import "github.com/passwordhash/cascade-hub-bot/internal/publication/models"

func entityToModel(entity slotEntity) models.Slot {
	return models.Slot{ID: entity.ID, ChannelID: entity.ChannelID, ScheduledAt: entity.ScheduledAt, State: models.State(entity.State), ChannelMessageID: entity.ChannelMessageID, LinkedMessageID: entity.LinkedMessageID, LeadUserID: entity.LeadUserID, PublishedAt: entity.PublishedAt, LinkAlertedAt: entity.LinkAlertedAt}
}

func entitiesToModels(entities []slotEntity) []models.Slot {
	result := make([]models.Slot, 0, len(entities))
	for _, entity := range entities {
		result = append(result, entityToModel(entity))
	}
	return result
}
