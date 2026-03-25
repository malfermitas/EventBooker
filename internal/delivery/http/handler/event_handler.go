package handler

import (
	"errors"
	"eventbooker/internal/delivery/http/middleware"
	"eventbooker/internal/service"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/wb-go/wbf/ginext"
)

type eventHandler struct {
	eventService service.EventService
}

// NewEventHandler builds an EventHandler and returns it as the interface type.
func NewEventHandler(eventService service.EventService) EventHandler {
	return eventHandler{eventService: eventService}
}

type createEventRequest struct {
	Title             string `json:"title" binding:"required"`
	StartAt           string `json:"start_at" binding:"required"`
	Capacity          int    `json:"capacity" binding:"required,gte=1"`
	BookingTTLSeconds int    `json:"booking_ttl_seconds" binding:"required,gte=1"`
	RequiresPayment   *bool  `json:"requires_payment"`
}

// CreateEvent handles POST /events and returns created event JSON with HTTP 201.
// Returns HTTP 400 for invalid input and HTTP 500 for service errors.
func (h eventHandler) CreateEvent(ctx *ginext.Context) {
	var req createEventRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	startAt, err := time.Parse(time.RFC3339, req.StartAt)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "start_at must be in RFC3339 format"})
		return
	}

	requiresPayment := true
	if req.RequiresPayment != nil {
		requiresPayment = *req.RequiresPayment
	}

	event, err := h.eventService.CreateEvent(ctx.Request.Context(), service.CreateEventInput{
		Title:             req.Title,
		StartAt:           startAt,
		Capacity:          req.Capacity,
		BookingTTLSeconds: req.BookingTTLSeconds,
		RequiresPayment:   requiresPayment,
	})
	if err != nil {
		writeServiceError(ctx, err)
		return
	}

	ctx.JSON(http.StatusCreated, eventResponseFromModel(event))
}

// ListEvents handles GET /events and returns a list of event JSON objects with HTTP 200.
// Returns HTTP 500 for service errors.
func (h eventHandler) ListEvents(ctx *ginext.Context) {
	events, err := h.eventService.ListEvents(ctx.Request.Context())
	if err != nil {
		writeServiceError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, eventResponsesFromModels(events))
}

// BookEvent handles POST /events/:id/book and returns booking JSON with HTTP 201.
// Returns HTTP 400 for invalid path/body input and HTTP 500 for service errors.
func (h eventHandler) BookEvent(ctx *ginext.Context) {
	eventID, ok := parseEventID(ctx)
	if !ok {
		return
	}

	userID, ok := currentUserID(ctx)
	if !ok {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": service.ErrUnauthorized.Error()})
		return
	}

	booking, err := h.eventService.BookEvent(ctx.Request.Context(), service.BookEventInput{
		EventID: eventID,
		UserID:  userID,
	})
	if err != nil {
		writeServiceError(ctx, err)
		return
	}

	ctx.JSON(http.StatusCreated, bookingResponseFromModel(booking))
}

// ConfirmBooking handles POST /events/:id/confirm and returns booking JSON with HTTP 200.
// Returns HTTP 400 for invalid path/body input and HTTP 500 for service errors.
func (h eventHandler) ConfirmBooking(ctx *ginext.Context) {
	eventID, ok := parseEventID(ctx)
	if !ok {
		return
	}

	userID, ok := currentUserID(ctx)
	if !ok {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": service.ErrUnauthorized.Error()})
		return
	}

	booking, err := h.eventService.ConfirmBooking(ctx.Request.Context(), service.ConfirmBookingInput{
		EventID: eventID,
		UserID:  userID,
	})
	if err != nil {
		writeServiceError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, bookingResponseFromModel(booking))
}

// GetEvent handles GET /events/:id and returns event details JSON with HTTP 200.
// Returns HTTP 400 for invalid event ID and HTTP 500 for service errors.
func (h eventHandler) GetEvent(ctx *ginext.Context) {
	eventID, ok := parseEventID(ctx)
	if !ok {
		return
	}

	details, err := h.eventService.GetEventDetails(ctx.Request.Context(), eventID)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, eventDetailsResponseFromModel(details))
}

// parseEventID reads :id from path and returns (eventID, true) on success.
// Returns (0, false) and writes HTTP 400 when the id is not a positive integer.
func parseEventID(ctx *ginext.Context) (int64, bool) {
	eventIDRaw := ctx.Param("id")
	eventID, err := strconv.ParseInt(eventIDRaw, 10, 64)
	if err != nil || eventID <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid event id"})
		return 0, false
	}

	return eventID, true
}

func currentUserID(ctx *ginext.Context) (int64, bool) {
	value, ok := ctx.Get(middleware.CurrentUserIDKey)
	if !ok {
		return 0, false
	}

	userID, ok := value.(int64)
	return userID, ok
}

func writeServiceError(ctx *ginext.Context, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidInput):
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrUnauthorized), errors.Is(err, service.ErrInvalidCredentials), errors.Is(err, service.ErrSessionNotFound), errors.Is(err, service.ErrSessionExpired), errors.Is(err, service.ErrSessionRevoked):
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrForbidden):
		ctx.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrEventNotFound), errors.Is(err, service.ErrUserNotFound), errors.Is(err, service.ErrBookingNotFound):
		ctx.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrNoSeatsAvailable), errors.Is(err, service.ErrBookingAlreadyExist), errors.Is(err, service.ErrBookingExpired), errors.Is(err, service.ErrEmailAlreadyExists):
		ctx.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	default:
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
