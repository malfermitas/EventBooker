package postgres

import (
	"context"

	"eventbooker/internal/domain/model"
	"eventbooker/internal/repository"
	pgxdriver "github.com/wb-go/wbf/dbpg/pgx-driver"
)

type EventRepository struct {
	db *pgxdriver.Postgres
}

func NewEventRepository(db *pgxdriver.Postgres) repository.EventRepository {
	return &EventRepository{db: db}
}

func (r *EventRepository) Create(ctx context.Context, event *model.Event) error {
	query := `
		INSERT INTO events (title, start_at, capacity, booking_ttl_seconds, requires_payment)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at
	`

	err := getQueryExecuter(ctx, r.db).QueryRow(
		ctx,
		query,
		event.Title,
		event.StartAt,
		event.Capacity,
		event.BookingTTLSeconds,
		event.RequiresPayment,
	).Scan(&event.ID, &event.CreatedAt)
	if err != nil {
		return err
	}

	return nil
}

func (r *EventRepository) GetByID(ctx context.Context, id int64) (*model.Event, error) {
	query := `
		SELECT id, title, start_at, capacity, booking_ttl_seconds, requires_payment, created_at
		FROM events
		WHERE id = $1
	`

	return scanEvent(getQueryExecuter(ctx, r.db).QueryRow(ctx, query, id))
}

func (r *EventRepository) List(ctx context.Context) ([]*model.Event, error) {
	query := `
		SELECT id, title, start_at, capacity, booking_ttl_seconds, requires_payment, created_at
		FROM events
		ORDER BY start_at ASC, id ASC
	`

	rows, err := getQueryExecuter(ctx, r.db).Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := make([]*model.Event, 0)
	for rows.Next() {
		event, scanErr := scanEvent(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		events = append(events, event)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return events, nil
}

func (r *EventRepository) LockByIDForUpdate(ctx context.Context, id int64) (*model.Event, error) {
	query := `
		SELECT id, title, start_at, capacity, booking_ttl_seconds, requires_payment, created_at
		FROM events
		WHERE id = $1
		FOR UPDATE
	`

	return scanEvent(getQueryExecuter(ctx, r.db).QueryRow(ctx, query, id))
}
