package postgres

import (
	"eventbooker/internal/domain/model"
	"github.com/jackc/pgx/v5"
)

func scanUser(row pgx.Row) (*model.User, error) {
	user := &model.User{}
	err := row.Scan(&user.ID, &user.Email, &user.Name, &user.PasswordHash, &user.Role, &user.CreatedAt)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func scanEvent(row pgx.Row) (*model.Event, error) {
	event := &model.Event{}
	err := row.Scan(
		&event.ID,
		&event.Title,
		&event.StartAt,
		&event.Capacity,
		&event.BookingTTLSeconds,
		&event.RequiresPayment,
		&event.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	return event, nil
}

func scanBooking(row pgx.Row) (*model.Booking, error) {
	booking := &model.Booking{}
	err := row.Scan(
		&booking.ID,
		&booking.EventID,
		&booking.UserID,
		&booking.Status,
		&booking.CreatedAt,
		&booking.ExpiresAt,
		&booking.ConfirmedAt,
		&booking.CancelReason,
	)
	if err != nil {
		return nil, err
	}

	return booking, nil
}

func scanRefreshToken(row pgx.Row) (*model.RefreshToken, error) {
	token := &model.RefreshToken{}
	err := row.Scan(
		&token.ID,
		&token.UserID,
		&token.TokenHash,
		&token.ExpiresAt,
		&token.CreatedAt,
		&token.RevokedAt,
		&token.ReplacedByTokenID,
		&token.UserAgent,
		&token.IPAddress,
	)
	if err != nil {
		return nil, err
	}

	return token, nil
}
