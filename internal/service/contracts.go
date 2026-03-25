package service

import (
	"context"
	"time"

	"eventbooker/internal/domain/model"
)

type RegisterInput struct {
	Email    string
	Name     string
	Password string
}

type LoginInput struct {
	Email     string
	Password  string
	UserAgent string
	IPAddress string
}

type RefreshInput struct {
	RefreshToken string
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

type CreateEventInput struct {
	Title             string
	StartAt           time.Time
	Capacity          int
	BookingTTLSeconds int
	RequiresPayment   bool
}

type BookEventInput struct {
	EventID int64
	UserID  int64
}

type ConfirmBookingInput struct {
	EventID int64
	UserID  int64
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
