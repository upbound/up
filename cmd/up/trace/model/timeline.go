package model

import (
	"time"
)

type TimeLine struct {
	// the duration of 10 characters
	Scale time.Duration
	// the time at the right of the timeline, or zero when following current time.
	FixedTime time.Time
}
