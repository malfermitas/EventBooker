package service

import (
	"context"
	"time"

	"eventbooker/internal/domain/model"
)

type RegisterInput struct {
	Email    string `validate:"required,email,max=254"`
	Name     string `validate:"required,min=2,max=100"`
	Password string `validate:"required,min=8,max=72"`
}

type LoginInput struct {
	Email     string `validate:"required,email,max=254"`
	Password  string `validate:"required,min=1,max=72"`
	UserAgent string
	IPAddress string
}

type RefreshInput struct {
	RefreshToken string `validate:"required"`
	UserAgent    string
	IPAddress    string
}

type LogoutInput struct {
	RefreshToken string
}

type AuthTokens struct {
	AccessToken string
	TokenType   string
	ExpiresIn   int64
}

type AuthClaims struct {
	UserID int64
	Email  string
	Role   model.UserRole
}

type LoginResult struct {
	Tokens *AuthTokens
	User   *model.User
}

type RefreshResult struct {
	Tokens *AuthTokens
	User   *model.User
}

type NotificationSender interface {
	ScheduleEmail(ctx context.Context, email, message string, sendAt time.Time) error
	ScheduleTelegram(ctx context.Context, userID int64, message string, sendAt time.Time) error
}

type CreateEventInput struct {
	Title             string    `validate:"required,min=3,max=200"`
	StartAt           time.Time `validate:"required,future_time"`
	Capacity          int       `validate:"required,gte=1,lte=100000"`
	BookingTTLSeconds int       `validate:"required,gte=1,lte=604800"`
	RequiresPayment   bool
}

type BookEventInput struct {
	EventID int64 `validate:"required,gt=0"`
	UserID  int64 `validate:"required,gt=0"`
}

type ConfirmBookingInput struct {
	EventID int64 `validate:"required,gt=0"`
	UserID  int64 `validate:"required,gt=0"`
}

type EventDetails struct {
	Event          *model.Event
	FreeSeats      int
	PendingCount   int64
	ConfirmedCount int64
	Bookings       []*model.Booking
}

type EventService interface {
	CreateEvent(ctx context.Context, input CreateEventInput) (*model.Event, error)
	ListEvents(ctx context.Context) ([]*model.Event, error)
	GetEventDetails(ctx context.Context, eventID int64) (*EventDetails, error)
	BookEvent(ctx context.Context, input BookEventInput) (*model.Booking, error)
	ConfirmBooking(ctx context.Context, input ConfirmBookingInput) (*model.Booking, error)
}

type AuthService interface {
	Register(ctx context.Context, input RegisterInput) (*model.User, error)
	Login(ctx context.Context, input LoginInput) (*LoginResult, string, error)
	Refresh(ctx context.Context, input RefreshInput) (*RefreshResult, string, error)
	Logout(ctx context.Context, input LogoutInput) error
	ParseAccessToken(token string) (*AuthClaims, error)
	GetUser(ctx context.Context, userID int64) (*model.User, error)
}

type BookingExpirationService interface {
	ExpirePendingBookings(ctx context.Context, now time.Time, limit int) (int64, error)
}
