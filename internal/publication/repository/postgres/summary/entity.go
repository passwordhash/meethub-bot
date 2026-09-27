package summary

import (
	"github.com/google/uuid"
	"github.com/passwordhash/cascade-hub-bot/internal/publication/models"
)

type partEntity struct {
	EditionSlotID  uuid.UUID `db:"edition_slot_id"`
	PartIndex      int       `db:"part_index"`
	Text           *string   `db:"text"`
	Status         string    `db:"status"`
	GroupMessageID *int64    `db:"group_message_id"`
}

func (e partEntity) toModel() (models.SummaryPart, bool) {
	if e.Text == nil {
		return models.SummaryPart{}, false
	}
	return models.SummaryPart{EditionSlotID: e.EditionSlotID, PartIndex: e.PartIndex, Text: *e.Text, Status: e.Status, GroupMessageID: e.GroupMessageID}, true
}
