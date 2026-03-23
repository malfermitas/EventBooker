package model

import "time"

type Event struct {
	ID                int64
	Title             string
	StartAt           time.Time
	Capacity          int
	BookingTTLSeconds int
	RequiresPayment   bool
	CreatedAt         time.Time
}
