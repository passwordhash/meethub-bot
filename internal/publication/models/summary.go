package models

import "github.com/google/uuid"

type SummaryPart struct {
	EditionSlotID  uuid.UUID
	PartIndex      int
	Text           string
	Status         string
	GroupMessageID *int64
}

type GetSummaryPartsFilters struct {
	EditionSlotID *uuid.UUID
}
