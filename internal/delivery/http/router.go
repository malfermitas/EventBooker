package http

import (
	"eventbooker/internal/delivery/http/handler"
	"eventbooker/internal/delivery/http/middleware"

	"github.com/wb-go/wbf/ginext"
)

// NewRouter wires event HTTP routes and returns configured ginext engine.
func NewRouter(authHandler handler.AuthHandler, eventHandler handler.EventHandler, authMiddleware middleware.AuthMiddleware) *ginext.Engine {
	router := ginext.New("")
	router.Use(ginext.Recovery())

	router.POST("/auth/register", authHandler.Register)
	router.POST("/auth/login", authHandler.Login)
	router.POST("/auth/refresh", authHandler.Refresh)
	router.POST("/auth/logout", authHandler.Logout)
	router.GET("/me", authMiddleware.RequireAuth(), authHandler.Me)

	router.GET("/events", eventHandler.ListEvents)
	router.POST("/events", eventHandler.CreateEvent)
	router.GET("/events/:id", eventHandler.GetEvent)
	router.POST("/events/:id/book", authMiddleware.RequireAuth(), eventHandler.BookEvent)
	router.POST("/events/:id/confirm", authMiddleware.RequireAuth(), eventHandler.ConfirmBooking)

	return router
}
