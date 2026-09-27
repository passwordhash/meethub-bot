package models

import "errors"

var (
	ErrSnapshotSourceUnavailable = errors.New("published snapshot source unavailable")
	ErrSnapshotSourceConflict    = errors.New("snapshot edition already belongs to another source")
)
