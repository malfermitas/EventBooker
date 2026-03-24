package repository

import (
	"context"
	"time"

	"eventbooker/internal/domain/model"
)

type UserRepository interface {
	Create(ctx context.Context, user *model.User) error
	GetByID(ctx context.Context, id int64) (*model.User, error)
	GetByEmail(ctx context.Context, email string) (*model.User, error)
}

type EventRepository interface {
	Create(ctx context.Context, event *model.Event) error
	GetByID(ctx context.Context, id int64) (*model.Event, error)
	List(ctx context.Context) ([]*model.Event, error)
	LockByIDForUpdate(ctx context.Context, id int64) (*model.Event, error)
}

type BookingRepository interface {
	Create(ctx context.Context, booking *model.Booking) error
	GetByID(ctx context.Context, id int64) (*model.Booking, error)
	GetActiveByEventAndUser(ctx context.Context, eventID, userID int64) (*model.Booking, error)
	ListByEventID(ctx context.Context, eventID int64) ([]*model.Booking, error)
	CountByEventAndStatuses(ctx context.Context, eventID int64, statuses []model.BookingStatus) (int64, error)
	ConfirmPendingByEventAndUser(ctx context.Context, eventID, userID int64, confirmedAt time.Time) (*model.Booking, error)
	ExpirePending(ctx context.Context, now time.Time, limit int) (int64, error)
}

type TxManager interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}
