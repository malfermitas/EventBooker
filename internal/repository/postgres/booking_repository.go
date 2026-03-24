package postgres

import (
	"context"
	"time"

	"eventbooker/internal/domain/model"
	"eventbooker/internal/repository"
	pgxdriver "github.com/wb-go/wbf/dbpg/pgx-driver"
)

type BookingRepository struct {
	db *pgxdriver.Postgres
}

func NewBookingRepository(db *pgxdriver.Postgres) repository.BookingRepository {
	return &BookingRepository{db: db}
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

	return scanBooking(getQueryExecuter(ctx, r.db).QueryRow(ctx, query, id))
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

	return scanBooking(getQueryExecuter(ctx, r.db).QueryRow(ctx, query, eventID, userID))
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
		return nil, err
	}
	defer rows.Close()

	bookings := make([]*model.Booking, 0)
	for rows.Next() {
		booking, scanErr := scanBooking(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		bookings = append(bookings, booking)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return bookings, nil
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

	return scanBooking(getQueryExecuter(ctx, r.db).QueryRow(ctx, query, eventID, userID, confirmedAt))
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
		RETURNING b.id
	`

	result, err := getQueryExecuter(ctx, r.db).Exec(ctx, query, now, limit)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected(), nil
}
