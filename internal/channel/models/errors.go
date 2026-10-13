package models

import "errors"

var ErrUpdatePending = errors.New("telegram update pending")
var ErrInvalidDailyTime = errors.New("invalid daily time")
var ErrInvalidPostOffset = errors.New("invalid post offset")
