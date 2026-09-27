package reports

import "github.com/passwordhash/cascade-hub-bot/internal/report/models"

func entityToModel(entity reportEntity) models.Report {
	return models.Report{SlotID: entity.SlotID, AuthorID: entity.UserID, Author: entity.Name, Text: entity.Text, MessageID: entity.MessageID, MessageAt: entity.MessageAt, EditAt: entity.EditAt, UpdatedAt: entity.UpdatedAt}
}

func snapshotEntityToModel(entity snapshotEntity) models.Snapshot {
	return models.Snapshot{ID: entity.ID, EditionSlotID: entity.EditionSlotID, SourceSlotID: entity.SourceSlotID, AuthorID: entity.UserID, Author: entity.Name, Text: entity.Text, MessageID: entity.MessageID, MessageAt: entity.MessageAt, EditAt: entity.EditAt, CreatedAt: entity.CreatedAt}
}

func snapshotEntitiesToModels(entities []snapshotEntity) []models.Snapshot {
	result := make([]models.Snapshot, 0, len(entities))
	for _, entity := range entities {
		result = append(result, snapshotEntityToModel(entity))
	}
	return result
}

func entitiesToModels(entities []reportEntity) []models.Report {
	result := make([]models.Report, 0, len(entities))
	for _, entity := range entities {
		result = append(result, entityToModel(entity))
	}
	return result
}
