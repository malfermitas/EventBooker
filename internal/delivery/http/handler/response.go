package handler

import (
	"eventbooker/internal/domain/model"
	"eventbooker/internal/service"
	"time"
)

type eventResponse struct {
	ID                int64     `json:"id"`
	Title             string    `json:"title"`
	StartAt           time.Time `json:"start_at"`
	Capacity          int       `json:"capacity"`
	BookingTTLSeconds int       `json:"booking_ttl_seconds"`
	RequiresPayment   bool      `json:"requires_payment"`
	CreatedAt         time.Time `json:"created_at"`
}

type bookingResponse struct {
	ID           int64               `json:"id"`
	EventID      int64               `json:"event_id"`
	UserID       int64               `json:"user_id"`
	Status       model.BookingStatus `json:"status"`
	CreatedAt    time.Time           `json:"created_at"`
	ExpiresAt    time.Time           `json:"expires_at"`
	ConfirmedAt  *time.Time          `json:"confirmed_at,omitempty"`
	CancelReason *string             `json:"cancel_reason,omitempty"`
}

type eventDetailsResponse struct {
	Event          eventResponse     `json:"event"`
	FreeSeats      int               `json:"free_seats"`
	PendingCount   int64             `json:"pending_count"`
	ConfirmedCount int64             `json:"confirmed_count"`
	Bookings       []bookingResponse `json:"bookings"`
}

func eventResponsesFromModels(events []*model.Event) []eventResponse {
	response := make([]eventResponse, 0, len(events))
	for _, event := range events {
		response = append(response, eventResponseFromModel(event))
	}

	return response
}

// eventResponseFromModel converts domain event model to HTTP response DTO.
// Returns populated eventResponse.
func eventResponseFromModel(event *model.Event) eventResponse {
	return eventResponse{
		ID:                event.ID,
		Title:             event.Title,
		StartAt:           event.StartAt,
		Capacity:          event.Capacity,
		BookingTTLSeconds: event.BookingTTLSeconds,
		RequiresPayment:   event.RequiresPayment,
		CreatedAt:         event.CreatedAt,
	}
}

// bookingResponseFromModel converts domain booking model to HTTP response DTO.
// Returns populated bookingResponse.
func bookingResponseFromModel(booking *model.Booking) bookingResponse {
	return bookingResponse{
		ID:           booking.ID,
		EventID:      booking.EventID,
		UserID:       booking.UserID,
		Status:       booking.Status,
		CreatedAt:    booking.CreatedAt,
		ExpiresAt:    booking.ExpiresAt,
		ConfirmedAt:  booking.ConfirmedAt,
		CancelReason: booking.CancelReason,
	}
}

// eventDetailsResponseFromModel converts service event details to HTTP response DTO.
// Returns eventDetailsResponse with mapped nested bookings.
func eventDetailsResponseFromModel(details *service.EventDetails) eventDetailsResponse {
	bookings := make([]bookingResponse, 0, len(details.Bookings))
	for _, booking := range details.Bookings {
		bookings = append(bookings, bookingResponseFromModel(booking))
	}

	return eventDetailsResponse{
		Event:          eventResponseFromModel(details.Event),
		FreeSeats:      details.FreeSeats,
		PendingCount:   details.PendingCount,
		ConfirmedCount: details.ConfirmedCount,
		Bookings:       bookings,
	}
}
