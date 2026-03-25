package handler

import "github.com/wb-go/wbf/ginext"

type EventHandler interface {
	CreateEvent(ctx *ginext.Context)
	ListEvents(ctx *ginext.Context)
	BookEvent(ctx *ginext.Context)
	ConfirmBooking(ctx *ginext.Context)
	GetEvent(ctx *ginext.Context)
}

type AuthHandler interface {
	Register(ctx *ginext.Context)
	Login(ctx *ginext.Context)
	Refresh(ctx *ginext.Context)
	Logout(ctx *ginext.Context)
	Me(ctx *ginext.Context)
}
