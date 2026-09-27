package summary

import "github.com/passwordhash/cascade-hub-bot/internal/publication/models"

func entitiesToModels(entities []partEntity) ([]models.SummaryPart, error) {
	parts := make([]models.SummaryPart, 0, len(entities))
	for _, entity := range entities {
		part, ok := entity.toModel()
		if !ok {
			return nil, errExpired
		}
		parts = append(parts, part)
	}
	return parts, nil
}
