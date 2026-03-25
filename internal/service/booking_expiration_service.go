package service

import (
	"context"
	"time"

	"eventbooker/internal/logging"
	"eventbooker/internal/repository"
)

type bookingExpirationService struct {
	logger            *logging.EventBookerLogger
	bookingRepository repository.BookingRepository
}

func NewBookingExpirationService(logger *logging.EventBookerLogger, bookingRepository repository.BookingRepository) BookingExpirationService {
	return &bookingExpirationService{logger: logger, bookingRepository: bookingRepository}
}

func (s *bookingExpirationService) ExpirePendingBookings(ctx context.Context, now time.Time, limit int) (int64, error) {
	requestLogger := s.logger.Ctx(ctx)
	if now.IsZero() {
		now = time.Now().UTC()
	}

	if limit <= 0 {
		limit = 100
	}

	expiredCount, err := s.bookingRepository.ExpirePending(ctx, now, limit)
	if err != nil {
		requestLogger.Errorw("failed to expire pending bookings", "error", err, "limit", limit)
		return 0, err
	}

	if expiredCount > 0 {
		requestLogger.Infow("expired pending bookings", "count", expiredCount)
	}

	return expiredCount, nil
}
