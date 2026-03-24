package service

import (
	"context"
	"time"

	"eventbooker/internal/domain/model"
)

type CreateEventInput struct {
	Title             string
	StartAt           time.Time
	Capacity          int
	BookingTTLSeconds int
	RequiresPayment   bool
}

type BookEventInput struct {
	EventID int64
	UserID  int64
}

type ConfirmBookingInput struct {
	EventID int64
	UserID  int64
}

type EventDetails struct {
	Event          *model.Event
	FreeSeats      int
	PendingCount   int64
	ConfirmedCount int64
	Bookings       []*model.Booking
}

type EventService interface {
	CreateEvent(ctx context.Context, input CreateEventInput) (*model.Event, error)
	GetEventDetails(ctx context.Context, eventID int64) (*EventDetails, error)
	BookEvent(ctx context.Context, input BookEventInput) (*model.Booking, error)
	ConfirmBooking(ctx context.Context, input ConfirmBookingInput) (*model.Booking, error)
}

type BookingExpirationService interface {
	ExpirePendingBookings(ctx context.Context, now time.Time, limit int) (int64, error)
}
