package http

import (
	"eventbooker/internal/delivery/http/handler"

	"github.com/wb-go/wbf/ginext"
)

// NewRouter wires event HTTP routes and returns configured ginext engine.
func NewRouter(eventHandler handler.EventHandler) *ginext.Engine {
	router := ginext.New("")
	router.Use(ginext.Recovery())

	router.GET("/events", eventHandler.ListEvents)
	router.POST("/events", eventHandler.CreateEvent)
	router.POST("/events/:id/book", eventHandler.BookEvent)
	router.POST("/events/:id/confirm", eventHandler.ConfirmBooking)
	router.GET("/events/:id", eventHandler.GetEvent)

	return router
}
