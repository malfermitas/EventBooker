package service

import "errors"

var (
	ErrInvalidInput        = errors.New("invalid input")
	ErrEventNotFound       = errors.New("event not found")
	ErrUserNotFound        = errors.New("user not found")
	ErrNoSeatsAvailable    = errors.New("no seats available")
	ErrBookingNotFound     = errors.New("booking not found")
	ErrBookingExpired      = errors.New("booking expired")
	ErrBookingAlreadyExist = errors.New("active booking already exists")
)
