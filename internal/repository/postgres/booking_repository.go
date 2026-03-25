package postgres

import (
	"context"
	"time"

	"eventbooker/internal/domain/model"
	"eventbooker/internal/logging"
	"eventbooker/internal/repository"

	pgxdriver "github.com/wb-go/wbf/dbpg/pgx-driver"
)

type BookingRepository struct {
	logger *logging.EventBookerLogger
	db     *pgxdriver.Postgres
}

func NewBookingRepository(logger *logging.EventBookerLogger, db *pgxdriver.Postgres) repository.BookingRepository {
	return &BookingRepository{logger: logger, db: db}
}

func (r *BookingRepository) Create(ctx context.Context, booking *model.Booking) error {
	query := `
		INSERT INTO bookings (event_id, user_id, status, expires_at, confirmed_at, cancel_reason)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at
	`

	err := getQueryExecuter(ctx, r.db).QueryRow(
		ctx,
		query,
		booking.EventID,
		booking.UserID,
		booking.Status,
		booking.ExpiresAt,
		booking.ConfirmedAt,
		booking.CancelReason,
	).Scan(&booking.ID, &booking.CreatedAt)
	if err != nil {
		r.logger.Ctx(ctx).Errorw("failed to create booking in postgres", "event_id", booking.EventID, "user_id", booking.UserID, "error", err)
		return err
	}

	return nil
}

func (r *BookingRepository) GetByID(ctx context.Context, id int64) (*model.Booking, error) {
	query := `
		SELECT id, event_id, user_id, status, created_at, expires_at, confirmed_at, cancel_reason
		FROM bookings
		WHERE id = $1
	`

	booking, err := scanBooking(getQueryExecuter(ctx, r.db).QueryRow(ctx, query, id))
	if err != nil {
		r.logger.Ctx(ctx).Debugw("failed to get booking by id from postgres", "booking_id", id, "error", err)
		return nil, err
	}

	return booking, nil
}

func (r *BookingRepository) GetActiveByEventAndUser(ctx context.Context, eventID, userID int64) (*model.Booking, error) {
	query := `
		SELECT id, event_id, user_id, status, created_at, expires_at, confirmed_at, cancel_reason
		FROM bookings
		WHERE event_id = $1
		  AND user_id = $2
		  AND status IN ('PENDING', 'CONFIRMED')
		ORDER BY created_at DESC, id DESC
		LIMIT 1
	`

	booking, err := scanBooking(getQueryExecuter(ctx, r.db).QueryRow(ctx, query, eventID, userID))
	if err != nil {
		r.logger.Ctx(ctx).Debugw("failed to get active booking from postgres", "event_id", eventID, "user_id", userID, "error", err)
		return nil, err
	}

	return booking, nil
}

func (r *BookingRepository) GetLatestByEventAndUser(ctx context.Context, eventID, userID int64) (*model.Booking, error) {
	query := `
		SELECT id, event_id, user_id, status, created_at, expires_at, confirmed_at, cancel_reason
		FROM bookings
		WHERE event_id = $1
		  AND user_id = $2
		ORDER BY created_at DESC, id DESC
		LIMIT 1
	`

	booking, err := scanBooking(getQueryExecuter(ctx, r.db).QueryRow(ctx, query, eventID, userID))
	if err != nil {
		r.logger.Ctx(ctx).Debugw("failed to get latest booking from postgres", "event_id", eventID, "user_id", userID, "error", err)
		return nil, err
	}

	return booking, nil
}

func (r *BookingRepository) ListByEventID(ctx context.Context, eventID int64) ([]*model.Booking, error) {
	query := `
		SELECT id, event_id, user_id, status, created_at, expires_at, confirmed_at, cancel_reason
		FROM bookings
		WHERE event_id = $1
		ORDER BY created_at DESC, id DESC
	`

	rows, err := getQueryExecuter(ctx, r.db).Query(ctx, query, eventID)
	if err != nil {
		r.logger.Ctx(ctx).Errorw("failed to list bookings by event from postgres", "event_id", eventID, "error", err)
		return nil, err
	}
	defer rows.Close()

	bookings := make([]*model.Booking, 0)
	for rows.Next() {
		booking, scanErr := scanBooking(rows)
		if scanErr != nil {
			r.logger.Ctx(ctx).Errorw("failed to scan booking row from postgres", "event_id", eventID, "error", scanErr)
			return nil, scanErr
		}
		bookings = append(bookings, booking)
	}

	if err = rows.Err(); err != nil {
		r.logger.Ctx(ctx).Errorw("failed while iterating booking rows from postgres", "event_id", eventID, "error", err)
		return nil, err
	}

	return bookings, nil
}

func (r *BookingRepository) GetStatsByEventID(ctx context.Context, eventID int64) (*repository.BookingStats, error) {
	query := `
		SELECT
			COUNT(*) FILTER (WHERE status = 'PENDING') AS pending_count,
			COUNT(*) FILTER (WHERE status = 'CONFIRMED') AS confirmed_count
		FROM bookings
		WHERE event_id = $1
	`

	stats := &repository.BookingStats{}
	err := getQueryExecuter(ctx, r.db).QueryRow(ctx, query, eventID).Scan(&stats.PendingCount, &stats.ConfirmedCount)
	if err != nil {
		r.logger.Ctx(ctx).Errorw("failed to load booking stats from postgres", "event_id", eventID, "error", err)
		return nil, err
	}

	return stats, nil
}

func (r *BookingRepository) CountByEventAndStatuses(ctx context.Context, eventID int64, statuses []model.BookingStatus) (int64, error) {
	if len(statuses) == 0 {
		return 0, nil
	}

	statusStrings := make([]string, 0, len(statuses))
	for _, status := range statuses {
		statusStrings = append(statusStrings, string(status))
	}

	query := `
		SELECT COUNT(*)
		FROM bookings
		WHERE event_id = $1
		  AND status = ANY($2::text[])
	`

	var count int64
	err := getQueryExecuter(ctx, r.db).QueryRow(ctx, query, eventID, statusStrings).Scan(&count)
	if err != nil {
		r.logger.Ctx(ctx).Errorw(
			"failed to count bookings by statuses from postgres",
			"event_id", eventID,
			"statuses", statusStrings,
			"error", err,
		)
		return 0, err
	}

	return count, nil
}

func (r *BookingRepository) ConfirmPendingByEventAndUser(ctx context.Context, eventID, userID int64, confirmedAt time.Time) (*model.Booking, error) {
	query := `
		UPDATE bookings
		SET status = 'CONFIRMED',
		    confirmed_at = $3,
		    cancel_reason = NULL
		WHERE event_id = $1
		  AND user_id = $2
		  AND status = 'PENDING'
		  AND expires_at > $3
		RETURNING id, event_id, user_id, status, created_at, expires_at, confirmed_at, cancel_reason
	`

	booking, err := scanBooking(getQueryExecuter(ctx, r.db).QueryRow(ctx, query, eventID, userID, confirmedAt))
	if err != nil {
		r.logger.Ctx(ctx).Debugw(
			"failed to confirm pending booking in postgres",
			"event_id", eventID,
			"user_id", userID,
			"error", err,
		)
		return nil, err
	}

	return booking, nil
}

func (r *BookingRepository) ExpirePending(ctx context.Context, now time.Time, limit int) (int64, error) {
	if limit <= 0 {
		limit = 100
	}

	query := `
		WITH to_expire AS (
			SELECT id
			FROM bookings
			WHERE status = 'PENDING'
			  AND expires_at <= $1
			ORDER BY expires_at ASC, id ASC
			LIMIT $2
			FOR UPDATE SKIP LOCKED
		)
		UPDATE bookings b
		SET status = 'EXPIRED',
		    cancel_reason = 'expired by deadline'
		FROM to_expire e
		WHERE b.id = e.id
	`

	result, err := getQueryExecuter(ctx, r.db).Exec(ctx, query, now, limit)
	if err != nil {
		r.logger.Ctx(ctx).Errorw(
			"failed to expire pending bookings in postgres",
			"limit", limit,
			"error", err,
		)
		return 0, err
	}

	return result.RowsAffected(), nil
}
