package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"eventbooker/internal/domain/model"
	"eventbooker/internal/logging"
	"eventbooker/internal/repository"

	"github.com/jackc/pgx/v5"
)

type eventService struct {
	logger            *logging.EventBookerLogger
	notifier          NotificationSender
	txManager         repository.TxManager
	userRepository    repository.UserRepository
	eventRepository   repository.EventRepository
	bookingRepository repository.BookingRepository
}

func NewEventService(
	logger *logging.EventBookerLogger,
	notifier NotificationSender,
	txManager repository.TxManager,
	userRepository repository.UserRepository,
	eventRepository repository.EventRepository,
	bookingRepository repository.BookingRepository,
) EventService {
	return &eventService{
		logger:            logger,
		notifier:          notifier,
		txManager:         txManager,
		userRepository:    userRepository,
		eventRepository:   eventRepository,
		bookingRepository: bookingRepository,
	}
}

func (s *eventService) CreateEvent(ctx context.Context, input CreateEventInput) (*model.Event, error) {
	requestLogger := s.logger.Ctx(ctx)
	normalizedInput := CreateEventInput{
		Title:             strings.TrimSpace(input.Title),
		StartAt:           input.StartAt,
		Capacity:          input.Capacity,
		BookingTTLSeconds: input.BookingTTLSeconds,
		RequiresPayment:   input.RequiresPayment,
	}

	if err := validateInput(normalizedInput); err != nil {
		requestLogger.Warnw("event creation rejected due to invalid input", "title", normalizedInput.Title, "start_at", normalizedInput.StartAt, "capacity", normalizedInput.Capacity, "error", err)
		return nil, err
	}

	event := &model.Event{
		Title:             normalizedInput.Title,
		StartAt:           normalizedInput.StartAt,
		Capacity:          normalizedInput.Capacity,
		BookingTTLSeconds: normalizedInput.BookingTTLSeconds,
		RequiresPayment:   normalizedInput.RequiresPayment,
	}

	if err := s.eventRepository.Create(ctx, event); err != nil {
		requestLogger.Errorw("failed to create event", "title", event.Title, "error", err)
		return nil, err
	}

	requestLogger.Infow("event created", "event_id", event.ID, "title", event.Title)
	return event, nil
}

func (s *eventService) ListEvents(ctx context.Context) ([]*model.Event, error) {
	events, err := s.eventRepository.List(ctx)
	if err != nil {
		s.logger.Ctx(ctx).Errorw("failed to list events", "error", err)
		return nil, err
	}

	return events, nil
}

func (s *eventService) GetEventDetails(ctx context.Context, eventID int64) (*EventDetails, error) {
	requestLogger := s.logger.Ctx(ctx)
	if eventID <= 0 {
		requestLogger.Warnw("event details rejected due to invalid event id", "event_id", eventID)
		return nil, ErrInvalidInput
	}

	event, err := s.eventRepository.GetByID(ctx, eventID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			requestLogger.Warnw("event not found", "event_id", eventID)
			return nil, ErrEventNotFound
		}
		requestLogger.Errorw("failed to load event details", "event_id", eventID, "error", err)
		return nil, err
	}

	stats, err := s.bookingRepository.GetStatsByEventID(ctx, eventID)
	if err != nil {
		requestLogger.Errorw("failed to load event booking stats", "event_id", eventID, "error", err)
		return nil, err
	}

	bookings, err := s.bookingRepository.ListByEventID(ctx, eventID)
	if err != nil {
		requestLogger.Errorw("failed to load event bookings", "event_id", eventID, "error", err)
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
	requestLogger := s.logger.Ctx(ctx)
	if err := validateInput(input); err != nil {
		requestLogger.Warnw("event booking rejected due to invalid input", "event_id", input.EventID, "user_id", input.UserID, "error", err)
		return nil, err
	}

	var booking *model.Booking
	var eventTitle string
	err := s.txManager.WithinTx(ctx, func(txCtx context.Context) error {
		event, err := s.eventRepository.LockByIDForUpdate(txCtx, input.EventID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrEventNotFound
			}
			return err
		}

		eventTitle = event.Title

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
		requestLogger.Warnw("event booking failed", "event_id", input.EventID, "user_id", input.UserID, "error", err)
		return nil, err
	}

	requestLogger.Infow("event booked", "event_id", input.EventID, "user_id", input.UserID, "booking_id", booking.ID, "status", booking.Status)
	s.notifyBookingCreated(ctx, input.UserID, eventTitle, booking)
	return booking, nil
}

func (s *eventService) ConfirmBooking(ctx context.Context, input ConfirmBookingInput) (*model.Booking, error) {
	requestLogger := s.logger.Ctx(ctx)
	if err := validateInput(input); err != nil {
		requestLogger.Warnw("booking confirmation rejected due to invalid input", "event_id", input.EventID, "user_id", input.UserID, "error", err)
		return nil, err
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
		requestLogger.Warnw("booking confirmation failed", "event_id", input.EventID, "user_id", input.UserID, "error", err)
		return nil, err
	}

	requestLogger.Infow("booking confirmed", "event_id", input.EventID, "user_id", input.UserID, "booking_id", confirmedBooking.ID)
	s.notifyBookingConfirmed(ctx, input.UserID, input.EventID, confirmedBooking)
	return confirmedBooking, nil
}

func (s *eventService) notifyBookingCreated(ctx context.Context, userID int64, eventTitle string, booking *model.Booking) {
	if s.notifier == nil {
		return
	}

	user, err := s.userRepository.GetByID(ctx, userID)
	if err != nil {
		s.logger.Ctx(ctx).Warnw("failed to load user for booking notification", "user_id", userID, "error", err)
		return
	}

	statusText := string(booking.Status)
	message := fmt.Sprintf(
		"Your booking for '%s' has been created. Booking #%d is currently %s and expires at %s UTC.",
		eventTitle,
		booking.ID,
		statusText,
		booking.ExpiresAt.UTC().Format(time.RFC3339),
	)

	s.sendBookingNotifications(ctx, user.ID, user.Email, message)
}

func (s *eventService) notifyBookingConfirmed(ctx context.Context, userID, eventID int64, booking *model.Booking) {
	if s.notifier == nil {
		return
	}

	user, err := s.userRepository.GetByID(ctx, userID)
	if err != nil {
		s.logger.Ctx(ctx).Warnw("failed to load user for booking confirmation notification", "user_id", userID, "error", err)
		return
	}

	event, err := s.eventRepository.GetByID(ctx, eventID)
	if err != nil {
		s.logger.Ctx(ctx).Warnw("failed to load event for booking confirmation notification", "event_id", eventID, "error", err)
		return
	}

	message := fmt.Sprintf(
		"Your booking for '%s' is confirmed. Booking #%d is confirmed and the event starts at %s UTC.",
		event.Title,
		booking.ID,
		event.StartAt.UTC().Format(time.RFC3339),
	)

	s.sendBookingNotifications(ctx, user.ID, user.Email, message)
}

func (s *eventService) sendBookingNotifications(ctx context.Context, userID int64, email, message string) {
	if err := s.notifier.ScheduleEmail(ctx, email, message, time.Now().UTC()); err != nil {
		s.logger.Ctx(ctx).Warnw("failed to schedule email notification", "user_id", userID, "error", err)
	}

	if err := s.notifier.ScheduleTelegram(ctx, userID, message, time.Now().UTC()); err != nil {
		s.logger.Ctx(ctx).Warnw("failed to schedule telegram notification", "user_id", userID, "error", err)
	}
}
