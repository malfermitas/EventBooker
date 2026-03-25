package repository

import (
	"context"
	"time"

	"eventbooker/internal/domain/model"
)

type BookingStats struct {
	PendingCount   int64
	ConfirmedCount int64
}

type UserRepository interface {
	Create(ctx context.Context, user *model.User) error
	GetByID(ctx context.Context, id int64) (*model.User, error)
	GetByEmail(ctx context.Context, email string) (*model.User, error)
}

type RefreshTokenRepository interface {
	Create(ctx context.Context, token *model.RefreshToken) error
	GetByTokenHash(ctx context.Context, tokenHash string) (*model.RefreshToken, error)
	RevokeByID(ctx context.Context, id int64, revokedAt time.Time) error
	RevokeAndReplace(ctx context.Context, id int64, revokedAt time.Time, replacedByTokenID int64) error
	RevokeAllByUserID(ctx context.Context, userID int64, revokedAt time.Time) error
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
	GetLatestByEventAndUser(ctx context.Context, eventID, userID int64) (*model.Booking, error)
	ListByEventID(ctx context.Context, eventID int64) ([]*model.Booking, error)
	GetStatsByEventID(ctx context.Context, eventID int64) (*BookingStats, error)
	CountByEventAndStatuses(ctx context.Context, eventID int64, statuses []model.BookingStatus) (int64, error)
	ConfirmPendingByEventAndUser(ctx context.Context, eventID, userID int64, confirmedAt time.Time) (*model.Booking, error)
	ExpirePending(ctx context.Context, now time.Time, limit int) (int64, error)
}

type TxManager interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}
