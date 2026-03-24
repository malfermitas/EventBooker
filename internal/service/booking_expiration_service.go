package service

import (
	"context"
	"time"

	"eventbooker/internal/repository"
)

type bookingExpirationService struct {
	bookingRepository repository.BookingRepository
}

func NewBookingExpirationService(bookingRepository repository.BookingRepository) BookingExpirationService {
	return &bookingExpirationService{bookingRepository: bookingRepository}
}

func (s *bookingExpirationService) ExpirePendingBookings(ctx context.Context, now time.Time, limit int) (int64, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}

	if limit <= 0 {
		limit = 100
	}

	return s.bookingRepository.ExpirePending(ctx, now, limit)
}
