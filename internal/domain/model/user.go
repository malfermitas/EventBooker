package model

import "time"

type User struct {
	ID        int64
	Email     string
	Name      string
	Role      UserRole
	CreatedAt time.Time
}
