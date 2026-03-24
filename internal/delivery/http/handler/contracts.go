package handler

import "github.com/wb-go/wbf/ginext"

type EventHandler interface {
	CreateEvent(ctx *ginext.Context)
	BookEvent(ctx *ginext.Context)
	ConfirmBooking(ctx *ginext.Context)
	GetEvent(ctx *ginext.Context)
}
