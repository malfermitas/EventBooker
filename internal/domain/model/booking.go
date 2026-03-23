package model

import "time"

type Booking struct {
	ID           int64
	EventID      int64
	UserID       int64
	Status       BookingStatus
	CreatedAt    time.Time
	ExpiresAt    time.Time
	ConfirmedAt  *time.Time
	CancelReason *string
}
