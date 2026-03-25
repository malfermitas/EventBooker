package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"eventbooker/internal/domain/model"
	"eventbooker/internal/repository"
	"github.com/jackc/pgx/v5"
)

type eventService struct {
	txManager         repository.TxManager
	userRepository    repository.UserRepository
	eventRepository   repository.EventRepository
	bookingRepository repository.BookingRepository
}

func NewEventService(
	txManager repository.TxManager,
	userRepository repository.UserRepository,
	eventRepository repository.EventRepository,
	bookingRepository repository.BookingRepository,
) EventService {
	return &eventService{
		txManager:         txManager,
		userRepository:    userRepository,
		eventRepository:   eventRepository,
		bookingRepository: bookingRepository,
	}
}

func (s *eventService) CreateEvent(ctx context.Context, input CreateEventInput) (*model.Event, error) {
	if strings.TrimSpace(input.Title) == "" || input.StartAt.IsZero() || input.Capacity <= 0 || input.BookingTTLSeconds <= 0 {
		return nil, ErrInvalidInput
	}

	event := &model.Event{
		Title:             strings.TrimSpace(input.Title),
		StartAt:           input.StartAt,
		Capacity:          input.Capacity,
		BookingTTLSeconds: input.BookingTTLSeconds,
		RequiresPayment:   input.RequiresPayment,
	}

	if err := s.eventRepository.Create(ctx, event); err != nil {
		return nil, err
	}

	return event, nil
}

func (s *eventService) ListEvents(ctx context.Context) ([]*model.Event, error) {
	return s.eventRepository.List(ctx)
}

func (s *eventService) GetEventDetails(ctx context.Context, eventID int64) (*EventDetails, error) {
	if eventID <= 0 {
		return nil, ErrInvalidInput
	}

	event, err := s.eventRepository.GetByID(ctx, eventID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrEventNotFound
		}
		return nil, err
	}

	stats, err := s.bookingRepository.GetStatsByEventID(ctx, eventID)
	if err != nil {
		return nil, err
	}

	bookings, err := s.bookingRepository.ListByEventID(ctx, eventID)
	if err != nil {
		return nil, err
	}

	occupied := int(stats.PendingCount + stats.ConfirmedCount)
	freeSeats := event.Capacity - occupied
	if freeSeats < 0 {
		freeSeats = 0
	}

	return &EventDetails{
		Event:          event,
		FreeSeats:      freeSeats,
		PendingCount:   stats.PendingCount,
		ConfirmedCount: stats.ConfirmedCount,
		Bookings:       bookings,
	}, nil
}

func (s *eventService) BookEvent(ctx context.Context, input BookEventInput) (*model.Booking, error) {
	if input.EventID <= 0 || input.UserID <= 0 {
		return nil, ErrInvalidInput
	}

	var booking *model.Booking
	err := s.txManager.WithinTx(ctx, func(txCtx context.Context) error {
		event, err := s.eventRepository.LockByIDForUpdate(txCtx, input.EventID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrEventNotFound
			}
			return err
		}

		if _, err = s.userRepository.GetByID(txCtx, input.UserID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrUserNotFound
			}
			return err
		}

		if _, err = s.bookingRepository.GetActiveByEventAndUser(txCtx, input.EventID, input.UserID); err == nil {
			return ErrBookingAlreadyExist
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}

		occupied, err := s.bookingRepository.CountByEventAndStatuses(
			txCtx,
			input.EventID,
			[]model.BookingStatus{model.BookingStatusPending, model.BookingStatusConfirmed},
		)
		if err != nil {
			return err
		}

		if occupied >= int64(event.Capacity) {
			return ErrNoSeatsAvailable
		}

		now := time.Now().UTC()
		expiresAt := now.Add(time.Duration(event.BookingTTLSeconds) * time.Second)

		booking = &model.Booking{
			EventID:   input.EventID,
			UserID:    input.UserID,
			Status:    model.BookingStatusPending,
			ExpiresAt: expiresAt,
		}

		if !event.RequiresPayment {
			booking.Status = model.BookingStatusConfirmed
			booking.ConfirmedAt = &now
		}

		if err = s.bookingRepository.Create(txCtx, booking); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return booking, nil
}

func (s *eventService) ConfirmBooking(ctx context.Context, input ConfirmBookingInput) (*model.Booking, error) {
	if input.EventID <= 0 || input.UserID <= 0 {
		return nil, ErrInvalidInput
	}

	var confirmedBooking *model.Booking
	err := s.txManager.WithinTx(ctx, func(txCtx context.Context) error {
		if _, err := s.eventRepository.LockByIDForUpdate(txCtx, input.EventID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrEventNotFound
			}
			return err
		}

		if _, err := s.userRepository.GetByID(txCtx, input.UserID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrUserNotFound
			}
			return err
		}

		booking, err := s.bookingRepository.ConfirmPendingByEventAndUser(txCtx, input.EventID, input.UserID, time.Now().UTC())
		if err == nil {
			confirmedBooking = booking
			return nil
		}

		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}

		latestBooking, activeErr := s.bookingRepository.GetLatestByEventAndUser(txCtx, input.EventID, input.UserID)
		if activeErr != nil {
			if errors.Is(activeErr, pgx.ErrNoRows) {
				return ErrBookingNotFound
			}
			return activeErr
		}

		if latestBooking.Status == model.BookingStatusConfirmed {
			confirmedBooking = latestBooking
			return nil
		}

		if latestBooking.Status == model.BookingStatusExpired {
			return ErrBookingExpired
		}

		return ErrBookingNotFound
	})
	if err != nil {
		return nil, err
	}

	return confirmedBooking, nil
}
