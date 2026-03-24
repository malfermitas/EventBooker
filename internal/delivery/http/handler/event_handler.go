package handler

import (
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

type bookingActionRequest struct {
	UserID int64 `json:"user_id" binding:"required,gt=0"`
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
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusCreated, eventResponseFromModel(event))
}

// BookEvent handles POST /events/:id/book and returns booking JSON with HTTP 201.
// Returns HTTP 400 for invalid path/body input and HTTP 500 for service errors.
func (h eventHandler) BookEvent(ctx *ginext.Context) {
	eventID, userID, ok := parseBookingActionInput(ctx)
	if !ok {
		return
	}

	booking, err := h.eventService.BookEvent(ctx.Request.Context(), service.BookEventInput{
		EventID: eventID,
		UserID:  userID,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusCreated, bookingResponseFromModel(booking))
}

// ConfirmBooking handles POST /events/:id/confirm and returns booking JSON with HTTP 200.
// Returns HTTP 400 for invalid path/body input and HTTP 500 for service errors.
func (h eventHandler) ConfirmBooking(ctx *ginext.Context) {
	eventID, userID, ok := parseBookingActionInput(ctx)
	if !ok {
		return
	}

	booking, err := h.eventService.ConfirmBooking(ctx.Request.Context(), service.ConfirmBookingInput{
		EventID: eventID,
		UserID:  userID,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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

// parseBookingActionInput parses event ID and booking action body.
// Returns (eventID, userID, true) on success, otherwise (0, 0, false) and HTTP 400.
func parseBookingActionInput(ctx *ginext.Context) (int64, int64, bool) {
	eventID, ok := parseEventID(ctx)
	if !ok {
		return 0, 0, false
	}

	var req bookingActionRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return 0, 0, false
	}

	return eventID, req.UserID, true
}
