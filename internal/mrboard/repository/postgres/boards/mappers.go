package boards

import "github.com/passwordhash/cascade-hub-bot/internal/mrboard/models"

func boardEntityToModel(entity boardEntity) models.Board {
	return models.Board{ChannelID: entity.ChannelID, MessageID: entity.MessageID, Body: entity.Body, Format: models.BoardFormat(entity.Format), UpdatedAt: entity.UpdatedAt}
}

func reviewerTrackEntityToModel(entity reviewerTrackEntity) models.ReviewerTrack {
	return models.ReviewerTrack{ProjectID: entity.ProjectID, IID: entity.IID, Username: entity.Username, State: models.ReviewerState(entity.State), ReviewedOnce: entity.ReviewedOnce, ChangedAt: entity.ChangedAt}
}

func sectionTrackEntityToModel(entity sectionTrackEntity) models.SectionTrack {
	return models.SectionTrack{ProjectID: entity.ProjectID, IID: entity.IID, Section: models.Section(entity.Section), Since: entity.Since}
}
